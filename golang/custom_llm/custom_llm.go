package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

type (
	// AudioContent represents audio content in a message
	AudioContent struct {
		InputAudio map[string]string `json:"input_audio"`
		Type       string            `json:"type"`
	}

	// ChatCompletionRequest represents a request for chat completion
	ChatCompletionRequest struct {
		// Audio response from the assistant
		Audio map[string]string `json:"audio,omitempty"`
		// Context information
		Context map[string]any `json:"context,omitempty"`
		// List of messages
		Messages []Message `json:"messages"`
		// List of modalities, defaults to ["text"]
		Modalities []string `json:"modalities"`
		// Name of the model to use
		Model string `json:"model,omitempty"`
		// Whether to call tools in parallel
		ParallelToolCalls bool `json:"parallel_tool_calls"`
		// Format of the response
		ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
		// Whether to use streaming response
		Stream bool `json:"stream"`
		// Options for streaming
		StreamOptions map[string]any `json:"stream_options,omitempty"`
		// Tool selection strategy, defaults to "auto"
		ToolChoice any `json:"tool_choice,omitempty"`
		// List of available tools
		Tools []Tool `json:"tools,omitempty"`
	}

	// ImageContent represents image content in a message
	ImageContent struct {
		ImageURL string `json:"image_url"`
		Type     string `json:"type"`
	}

	// Message represents a message in the chat
	Message struct {
		Audio      map[string]string `json:"audio,omitempty"`
		Content    any               `json:"content"`
		Role       string            `json:"role"`
		ToolCallID string            `json:"tool_call_id,omitempty"`
		ToolCalls  []map[string]any  `json:"tool_calls,omitempty"`
	}

	// ResponseFormat represents the format of the response
	ResponseFormat struct {
		JSONSchema map[string]string `json:"json_schema,omitempty"`
		Type       string            `json:"type"`
	}

	// TextContent represents text content in a message
	TextContent struct {
		Text string `json:"text"`
		Type string `json:"type"`
	}

	// Tool represents a tool that can be used by the model
	Tool struct {
		Function ToolFunction `json:"function"`
		Type     string       `json:"type"`
	}

	// ToolChoice represents the choice of tool to use
	ToolChoice struct {
		Function map[string]any `json:"function,omitempty"`
		Type     string         `json:"type"`
	}

	// ToolFunction represents a function definition for a tool
	ToolFunction struct {
		Description string         `json:"description,omitempty"`
		Name        string         `json:"name"`
		Parameters  map[string]any `json:"parameters,omitempty"`
		Strict      bool           `json:"strict"`
	}
)

var waitingMessages = []string{
	"Just a moment, I'm thinking...",
	"Let me think about that for a second...",
	"Good question, let me find out...",
}

// Server represents the chat completion server
type Server struct {
	client *openai.Client
	logger *slog.Logger
}

// NewServer creates a new server instance
func NewServer(apiKey string) *Server {
	return &Server{
		client: openai.NewClient(apiKey),
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// handleAudioChatCompletion handles the audio chat completion endpoint
func (s *Server) handleAudioChatCompletion(c *gin.Context) {
	var request ChatCompletionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.sendError(c, http.StatusBadRequest, err)
		return
	}

	if !request.Stream {
		s.sendError(c, http.StatusBadRequest, fmt.Errorf("chat completions require streaming"))
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")

	// Read text and audio files
	textContent, err := s.readTextFile("./file.txt")
	if err != nil {
		s.logger.Error("Failed to read text file", "err", err)
		s.sendError(c, http.StatusInternalServerError, err)
		return
	}

	sampleRate := 16000 // Example sample rate
	durationMs := 40    // 40ms chunks
	audioChunks, err := s.readPCMFile("./file.pcm", sampleRate, durationMs)
	if err != nil {
		s.logger.Error("Failed to read PCM file", "err", err)
		s.sendError(c, http.StatusInternalServerError, err)
		return
	}

	// Send text content
	audioID := uuid.New().String()
	textMessage := map[string]any{
		"id": uuid.New().String(),
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"audio": map[string]any{
						"id":         audioID,
						"transcript": textContent,
					},
				},
				"finish_reason": nil,
			},
		},
	}

	data, _ := json.Marshal(textMessage)
	c.SSEvent("data", string(data))

	// Send audio chunks
	for _, chunk := range audioChunks {
		audioMessage := map[string]any{
			"id": uuid.New().String(),
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"audio": map[string]any{
							"id":   audioID,
							"data": base64.StdEncoding.EncodeToString(chunk),
						},
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ := json.Marshal(audioMessage)
		c.SSEvent("data", string(data))
	}

	c.SSEvent("data", "[DONE]")
}

// handleChatCompletion handles the chat completion endpoint
func (s *Server) handleChatCompletion(c *gin.Context) {
	var request ChatCompletionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		s.sendError(c, http.StatusBadRequest, err)
		return
	}

	if !request.Stream {
		s.sendError(c, http.StatusBadRequest, fmt.Errorf("chat completions require streaming"))
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")

	responseChan := make(chan any, 100)
	errorChan := make(chan error, 1)

	go func() {
		messages := make([]openai.ChatCompletionMessage, len(request.Messages))
		for i, msg := range request.Messages {
			if strContent, ok := msg.Content.(string); ok {
				messages[i] = openai.ChatCompletionMessage{
					Role:    msg.Role,
					Content: strContent,
				}
			}
		}

		req := openai.ChatCompletionRequest{
			Model:    request.Model,
			Messages: messages,
			Stream:   true,
		}

		if len(request.Tools) > 0 {
			tools := make([]openai.Tool, len(request.Tools))

			for i, tool := range request.Tools {
				tools[i] = openai.Tool{
					Type: openai.ToolTypeFunction,
					Function: &openai.FunctionDefinition{
						Name:        tool.Function.Name,
						Description: tool.Function.Description,
						Parameters:  tool.Function.Parameters,
					},
				}
			}

			req.Tools = tools
		}

		stream, err := s.client.CreateChatCompletionStream(c.Request.Context(), req)
		if err != nil {
			errorChan <- err
			return
		}

		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err == io.EOF {
				break
			}

			if err != nil {
				errorChan <- err
				return
			}

			responseChan <- response
		}

		close(responseChan)
	}()

	for {
		select {
		case chunk, ok := <-responseChan:
			if !ok {
				c.SSEvent("data", "[DONE]")
				return
			}

			data, _ := json.Marshal(chunk)
			c.SSEvent("data", string(data))
		case err := <-errorChan:
			s.logger.Error("Error in chat completion stream", "err", err)
			s.sendError(c, http.StatusInternalServerError, err)
			return
		}
	}
}

// handleRAGChatCompletion handles the RAG chat completion endpoint
func (s *Server) handleRAGChatCompletion(c *gin.Context) {
	var request ChatCompletionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		s.sendError(c, http.StatusBadRequest, err)
		return
	}

	if !request.Stream {
		s.sendError(c, http.StatusBadRequest, fmt.Errorf("chat completions require streaming"))
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")

	// First send a "please wait" prompt
	waitingMsg := map[string]any{
		"id": "waiting_msg",
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": waitingMessages[rand.Intn(len(waitingMessages))],
				},
				"finish_reason": nil,
			},
		},
	}
	data, _ := json.Marshal(waitingMsg)
	c.SSEvent("data", string(data))

	// Perform RAG retrieval
	retrievedContext, err := s.performRAGRetrieval(request.Messages)
	if err != nil {
		s.logger.Error("Failed to perform RAG retrieval", "err", err)
		s.sendError(c, http.StatusInternalServerError, err)
		return
	}

	// Adjust messages
	refactedMessages := s.refactMessages(retrievedContext, request.Messages)

	// Convert messages to OpenAI format
	messages := make([]openai.ChatCompletionMessage, len(refactedMessages))
	for i, msg := range refactedMessages {
		if strContent, ok := msg.Content.(string); ok {
			messages[i] = openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: strContent,
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    request.Model,
		Messages: messages,
		Stream:   true,
	}

	stream, err := s.client.CreateChatCompletionStream(c.Request.Context(), req)
	if err != nil {
		s.sendError(c, http.StatusInternalServerError, err)
		return
	}

	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.sendError(c, http.StatusInternalServerError, err)
			return
		}
		data, _ := json.Marshal(response)
		c.SSEvent("data", string(data))
	}

	c.SSEvent("data", "[DONE]")
}

// performRAGRetrieval retrieves relevant content from the knowledge base message list using the RAG model.
//
// messages: contains the original message list.
//
// Return the retrieved text content and any error that occurred during retrieval.
func (s *Server) performRAGRetrieval(messages []Message) (string, error) {
	// TODO: Implement actual RAG retrieval logic
	// You may need to take the first or the last message from the messages as the query, depending on your specific needs
	// Then send the query to the RAG model to retrieve relevant content

	// Return retrieval results
	return "This is relevant content retrieved from the knowledge base.", nil
}

// readPCMFile reads a PCM file and returns audio chunks.
//
// filePath:   specifies the path to the PCM file.
// sampleRate: specifies the sample rate of the audio.
// durationMs: specifies the duration of each audio chunk in milliseconds.
//
// Return a list of audio chunks and any error that occurred during reading.
func (s *Server) readPCMFile(filePath string, sampleRate int, durationMs int) ([][]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM file: %w", err)
	}

	chunkSize := int(float64(sampleRate) * 2 * float64(durationMs) / 1000.0)
	if chunkSize == 0 {
		return nil, fmt.Errorf("invalid chunk size: sample rate %d, duration %dms", sampleRate, durationMs)
	}

	chunks := make([][]byte, 0, len(data)/chunkSize+1)

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}

	return chunks, nil
}

// readTextFile reads a text file and returns its content.
//
// filePath: specifies the path to the text file.
//
// Return the content of the text file and any error that occurred during reading.
func (s *Server) readTextFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read text file: %w", err)
	}
	return string(data), nil
}

// refactMessages adjusts the message list by adding the retrieved context to the original message list.
//
// context:  contains the retrieved context.
// messages: contains the original message list.
//
// Return the adjusted message list.
func (s *Server) refactMessages(context string, messages []Message) []Message {
	// TODO: Implement actual message adjustment logic
	// This should add the retrieved context to the original message list

	// For now, just return the original messages
	return messages
}

// sendError sends an error response to the client
func (s *Server) sendError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"detail": err.Error()})
}

// setupRoutes sets up all the routes for the server
func (s *Server) setupRoutes(r *gin.Engine) {
	r.POST("/audio/chat/completions", s.handleAudioChatCompletion)
	r.POST("/chat/completions", s.handleChatCompletion)
	r.POST("/rag/chat/completions", s.handleRAGChatCompletion)
}

func main() {
	// Initialize server
	server := NewServer(os.Getenv("YOUR_LLM_API_KEY"))

	// Initialize Gin router
	r := gin.Default()

	// Setup routes
	server.setupRoutes(r)

	// Start server
	r.Run(":8000")
}
