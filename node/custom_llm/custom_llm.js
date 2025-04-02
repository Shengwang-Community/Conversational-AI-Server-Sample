const express = require('express');
const cors = require('cors');
const morgan = require('morgan');
const dotenv = require('dotenv');
const OpenAI = require('openai');
const fs = require('fs').promises;
const { randomUUID } = require('crypto');

// Load environment variables
dotenv.config();

// Initialize OpenAI client
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY || 'your-api-key-here',
});

// Initialize Express app
const app = express();
const port = process.env.PORT || 8000;

// Configure logging
const logger = {
  info: (message) => console.log(`INFO: ${message}`),
  debug: (message) => console.log(`DEBUG: ${message}`),
  error: (message, error) => console.error(`ERROR: ${message}`, error),
};

// Middleware
app.use(cors());
app.use(morgan('dev'));
app.use(express.json());

// Health check endpoint
app.get('/ping', (req, res) => {
  res.json({ message: 'pong' });
});

// Root endpoint
app.get('/', (req, res) => {
  res.json({
    message: 'Welcome to a simple Custom LLM server for Agora Convo AI Engine!',
    endpoints: [
      '/chat/completions',
      '/rag/chat/completions',
      '/audio/chat/completions',
    ],
  });
});

// Basic Chat Completions API
app.post('/chat/completions', async (req, res) => {
  try {
    logger.info(`Received request: ${JSON.stringify(req.body)}`);

    const {
      model = 'gpt-4o-mini',
      messages,
      modalities = ['text'],
      tools,
      tool_choice,
      response_format,
      audio,
      stream = true,
      stream_options,
    } = req.body;

    if (!messages) {
      return res
        .status(400)
        .json({ detail: 'Missing messages in request body' });
    }

    if (!stream) {
      return res
        .status(400)
        .json({ detail: 'chat completions require streaming' });
    }

    // Set SSE headers
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');

    // Create OpenAI streaming completion
    const completion = await openai.chat.completions.create({
      model,
      messages,
      tools: tools ? tools : undefined,
      tool_choice: tools && tool_choice ? tool_choice : undefined,
      response_format,
      stream: true,
    });

    // Stream the response
    for await (const chunk of completion) {
      logger.debug(`Received chunk: ${JSON.stringify(chunk)}`);
      res.write(`data: ${JSON.stringify(chunk)}\n\n`);
    }

    // End the stream
    res.write('data: [DONE]\n\n');
    res.end();
  } catch (error) {
    logger.error('Chat completion error:', error);

    if (!res.headersSent) {
      const errorDetail = `${error.message}\n${error.stack || ''}`;
      return res.status(500).json({ detail: errorDetail });
    }

    res.write(`data: ${JSON.stringify({ error: error.message })}\n\n`);
    res.write('data: [DONE]\n\n');
    res.end();
  }
});

/**
 * Retrieves relevant content from the knowledge base
 * @param {Array} messages - Original message list
 * @returns {Promise<string>} Retrieved text content
 */
async function performRagRetrieval(messages) {
  // TODO: Implement actual RAG retrieval logic
  // You may need to take the first or the last message from the messages as the query
  // Then send the query to the RAG model to retrieve relevant content

  // Return retrieval results
  return 'This is relevant content retrieved from the knowledge base.';
}

/**
 * Adjusts the message list by adding the retrieved context
 * @param {string} context - Retrieved context
 * @param {Array} messages - Original message list
 * @returns {Array} Adjusted message list
 */
function refactMessages(context, messages) {
  // TODO: Implement actual message adjustment logic
  // This should add the retrieved context to the original message list

  // For now, we'll add a system message with the context
  return [
    {
      role: 'system',
      content: `You have access to the following knowledge: ${context}. Answer questions using this data.`,
    },
    ...messages,
  ];
}

// Waiting messages for RAG
const waitingMessages = [
  "Just a moment, I'm thinking...",
  'Let me think about that for a second...',
  'Good question, let me find out...',
];

// RAG-enhanced Chat Completions API
app.post('/rag/chat/completions', async (req, res) => {
  try {
    logger.info(`Received RAG request: ${JSON.stringify(req.body)}`);

    const {
      model = 'gpt-4',
      messages,
      modalities = ['text'],
      tools,
      tool_choice,
      response_format,
      audio,
      stream = true,
      stream_options,
    } = req.body;

    if (!messages) {
      return res
        .status(400)
        .json({ detail: 'Missing messages in request body' });
    }

    if (!stream) {
      return res
        .status(400)
        .json({ detail: 'chat completions require streaming' });
    }

    // Set SSE headers
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');

    // First send a "please wait" prompt
    const waitingMessage = {
      id: 'waiting_msg',
      choices: [
        {
          index: 0,
          delta: {
            role: 'assistant',
            content:
              waitingMessages[
                Math.floor(Math.random() * waitingMessages.length)
              ],
          },
          finish_reason: null,
        },
      ],
    };

    res.write(`data: ${JSON.stringify(waitingMessage)}\n\n`);

    // Perform RAG retrieval
    const retrievedContext = await performRagRetrieval(messages);

    // Adjust messages with retrieved context
    const ragMessages = refactMessages(retrievedContext, messages);

    // Create OpenAI streaming completion with RAG context
    const completion = await openai.chat.completions.create({
      model,
      messages: ragMessages,
      tools: tools ? tools : undefined,
      tool_choice: tools && tool_choice ? tool_choice : undefined,
      response_format,
      stream: true,
    });

    // Stream the response
    for await (const chunk of completion) {
      logger.debug(`Received RAG chunk: ${JSON.stringify(chunk)}`);
      res.write(`data: ${JSON.stringify(chunk)}\n\n`);
    }

    // End the stream
    res.write('data: [DONE]\n\n');
    res.end();
  } catch (error) {
    logger.error('RAG chat completion error:', error);

    if (!res.headersSent) {
      const errorDetail = `${error.message}\n${error.stack || ''}`;
      return res.status(500).json({ detail: errorDetail });
    }

    res.write(`data: ${JSON.stringify({ error: error.message })}\n\n`);
    res.write('data: [DONE]\n\n');
    res.end();
  }
});

/**
 * Reads a text file and returns the content
 * @param {string} filePath - Path to the text file
 * @returns {Promise<string>} Content of the text file
 */
async function readTextFile(filePath) {
  try {
    const content = await fs.readFile(filePath, 'utf8');
    return content;
  } catch (error) {
    logger.error(`Failed to read text file: ${filePath}`, error);
    throw error;
  }
}

/**
 * Reads a PCM file and returns audio chunks
 * @param {string} filePath - Path to the PCM file
 * @param {number} sampleRate - Sample rate of the audio
 * @param {number} durationMs - Duration of each chunk in milliseconds
 * @returns {Promise<Buffer[]>} List of audio chunks
 */
async function readPCMFile(filePath, sampleRate, durationMs) {
  try {
    const content = await fs.readFile(filePath);
    const chunkSize = Math.floor(sampleRate * 2 * (durationMs / 1000));
    const chunks = [];

    for (let i = 0; i < content.length; i += chunkSize) {
      chunks.push(content.slice(i, i + chunkSize));
    }

    return chunks;
  } catch (error) {
    logger.error(`Failed to read PCM file: ${filePath}`, error);
    throw error;
  }
}

// Audio Chat Completions API
app.post('/audio/chat/completions', async (req, res) => {
  try {
    logger.info(`Received audio request: ${JSON.stringify(req.body)}`);

    const {
      model = 'gpt-4',
      messages,
      modalities = ['text'],
      tools,
      tool_choice,
      response_format,
      audio,
      stream = true,
      stream_options,
    } = req.body;

    if (!messages) {
      return res
        .status(400)
        .json({ detail: 'Missing messages in request body' });
    }

    if (!stream) {
      return res
        .status(400)
        .json({ detail: 'chat completions require streaming' });
    }

    // Set SSE headers
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');

    // In a real implementation, you'd need to actually check for these files
    // and have proper error handling. This is simplified for the example.
    // You might want to use a try/catch with fs.access() to verify files exist.
    const textFilePath = './file.txt';
    const pcmFilePath = './file.pcm';
    const sampleRate = 16000; // Example sample rate
    const durationMs = 40; // 40ms chunks

    try {
      // Read text content and audio file
      const textContent = await readTextFile(textFilePath);
      const audioChunks = await readPCMFile(
        pcmFilePath,
        sampleRate,
        durationMs
      );

      // Generate audio ID for this response
      const audioId = randomUUID();

      // Send text content (transcript)
      const textMessage = {
        id: randomUUID(),
        choices: [
          {
            index: 0,
            delta: {
              audio: {
                id: audioId,
                transcript: textContent,
              },
            },
            finish_reason: null,
          },
        ],
      };

      res.write(`data: ${JSON.stringify(textMessage)}\n\n`);

      // Send audio chunks
      for (const chunk of audioChunks) {
        const audioMessage = {
          id: randomUUID(),
          choices: [
            {
              index: 0,
              delta: {
                audio: {
                  id: audioId,
                  data: chunk.toString('base64'),
                },
              },
              finish_reason: null,
            },
          ],
        };

        res.write(`data: ${JSON.stringify(audioMessage)}\n\n`);

        // Add a small delay between chunks to simulate streaming
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    } catch (error) {
      // If files don't exist or there's an error reading them,
      // we'll simulate the audio response
      logger.error(
        'Error reading audio files, using simulated response',
        error
      );

      const audioId = randomUUID();
      const simulatedTranscript =
        "This is a simulated audio response because actual audio files weren't found.";

      // Send simulated transcript
      const textMessage = {
        id: randomUUID(),
        choices: [
          {
            index: 0,
            delta: {
              audio: {
                id: audioId,
                transcript: simulatedTranscript,
              },
            },
            finish_reason: null,
          },
        ],
      };

      res.write(`data: ${JSON.stringify(textMessage)}\n\n`);

      // Send simulated audio chunks
      for (let i = 0; i < 5; i++) {
        const randomData = Buffer.from(
          Array(40)
            .fill(0)
            .map(() => Math.floor(Math.random() * 256))
        );

        const audioMessage = {
          id: randomUUID(),
          choices: [
            {
              index: 0,
              delta: {
                audio: {
                  id: audioId,
                  data: randomData.toString('base64'),
                },
              },
              finish_reason: null,
            },
          ],
        };

        res.write(`data: ${JSON.stringify(audioMessage)}\n\n`);
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    }

    // End the stream
    res.write('data: [DONE]\n\n');
    res.end();
  } catch (error) {
    logger.error('Audio chat completion error:', error);

    if (!res.headersSent) {
      const errorDetail = `${error.message}\n${error.stack || ''}`;
      return res.status(500).json({ detail: errorDetail });
    }

    res.write(`data: ${JSON.stringify({ error: error.message })}\n\n`);
    res.write('data: [DONE]\n\n');
    res.end();
  }
});

// Start server
app.listen(port, () => {
  logger.info(`Server running on port ${port}`);
});
