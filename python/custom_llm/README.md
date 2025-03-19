# 🌟 Custom LLm Sample Code for Python

> 声网对话式 AI 引擎支持自定义大语言模型（LLM）功能，您可以参考此项目代码自定义实现大语言模型功能。

本文档是实现自定义大语言模型功能的 Python 示例代码

## 🚀 一、快速开始

### 1.1 环境准备

- Python 3.10+

### 1.2 安装依赖

```bash
pip install -r requirements.txt
```

### 1.3 运行示例代码

```bash
python3 custom_llm.py
```

## 📖 二、功能说明

### 2.1 基本的自定义大语言模型

> 要成功接入声网对话式 AI 引擎，你的自定义大模型服务必须提供一个与 OpenAI Chat Completions API 兼容的接口。

实现逻辑参考函数`create_chat_completion` 实现。

### 2.2 实现检索增强的自定义大语言模型

>如果你想提升智能体响应的准确性和相关性，可以利用检索增强生成（RAG）功能，让你的自定义大模型从特定知识库中检索信息，再将检索结果作为上下文提供给大模型生成回答。

实现逻辑参考函数`create_rag_chat_completion` 实现。


### 2.3 实现多模态的自定义大语言模型

前提准备：
 - 在当前目录放置`file.pcm`文件，Sample Rate 为 16000，16bit，单声道，格式为 PCM。
 - 在当前目录放置`file.txt`文件，内容为上述音频文件的文本转写结果。

实现逻辑参考函数`create_audio_chat_completion` 实现。

## 📚 三、相关资源

- 📖 查看我们的 [对话式 AI 引擎文档](https://doc.shengwang.cn/doc/convoai/restful/landing-page) 了解更多详情
- 🧩 访问 [Agora SDK 示例](https://github.com/AgoraIO) 获取更多教程和示例代码
- 👥 在 [Agora 开发者社区](https://github.com/AgoraIO-Community) 探索开发者社区管理的优质代码仓库

## 💡 四、问题反馈

如果您在集成过程中遇到任何问题或有改进建议：

- 🤖 可通过 [声网支持](https://ticket.shengwang.cn/form?type_id=&sdk_product=&sdk_platform=&sdk_version=&current=0&project_id=&call_id=&channel_name=) 获取智能客服帮助或联系技术支持人员

## 📜 五、许可证

本项目采用 MIT 许可证 (The MIT License)。