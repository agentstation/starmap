# DeepSeek-Coder-V2-Instruct
  
[Catalog](../../../..) / [Providers](../../..) / [DeepSeek](../..) / **DeepSeek-Coder-V2-Instruct**


Code-specialized language model fine-tuned for programming tasks,  code generation, and software development assistance

  
  
## 📋 Overview
  
- **ID**: `deepseek-coder-v2-instruct`
- **Provider**: [DeepSeek](../)
- **Authors**: 
- **Release Date**: 2024-06-20
- **Knowledge Cutoff**: 2024-06-01
- **Open Weights**: true
- **Context Window**: 163k tokens
- **Max Output**: 8k tokens
- **Parameters**: 236B
  
## 🔬 Technical Specifications
  
**Sampling Controls:** ![Temperature](https://img.shields.io/badge/temperature-supported-red) ![Top-P](https://img.shields.io/badge/top__p-supported-red)

**Repetition Controls:** ![Frequency](https://img.shields.io/badge/frequency__penalty-supported-purple) ![Presence](https://img.shields.io/badge/presence__penalty-supported-purple) ![Repetition](https://img.shields.io/badge/repetition__penalty-supported-purple)

**Advanced Features:** ![Seed](https://img.shields.io/badge/seed-deterministic-green)
  
  
## 🎯 Capabilities
  
### Feature Overview
  
![Supports text generation and processing](https://img.shields.io/badge/text-✓-blue) ![Supported input modalities](https://img.shields.io/badge/input-text-teal) ![Supported output modalities](https://img.shields.io/badge/output-text-cyan) ![Can invoke and call tools in responses](https://img.shields.io/badge/tool__calls-✓-yellow) ![Accepts tool definitions in requests](https://img.shields.io/badge/tools-✓-yellow) ![Supports tool choice strategies (auto/none/required)](https://img.shields.io/badge/tool__choice-✓-yellow) ![Supports basic reasoning](https://img.shields.io/badge/reasoning-✓-lime) ![Temperature sampling control](https://img.shields.io/badge/temperature-core-red) ![Nucleus sampling (top-p)](https://img.shields.io/badge/top__p-core-red) ![Maximum token limit](https://img.shields.io/badge/max__tokens-core-blue) ![Stop sequences](https://img.shields.io/badge/stop-core-blue) ![Frequency penalty](https://img.shields.io/badge/frequency__penalty-core-purple) ![Presence penalty](https://img.shields.io/badge/presence__penalty-core-purple) ![Repetition penalty](https://img.shields.io/badge/repetition__penalty-advanced-purple) ![Deterministic seeding](https://img.shields.io/badge/seed-advanced-green) ![Alternative response formats](https://img.shields.io/badge/format__response-✓-cyan) ![JSON schema validation](https://img.shields.io/badge/structured__outputs-✓-cyan) ![Response streaming](https://img.shields.io/badge/streaming-✓-cyan)
  
  
### Input/Output Modalities
  
| Direction | Text | Image | Audio | Video | PDF |
|---------|---------|---------|---------|---------|---------|
| **Input** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Output** | ✅ | ❌ | ❌ | ❌ | ❌ |

  
### Core Features
  
| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |
|---------|---------|---------|---------|---------|
| ✅ | ✅ | ✅ | ❌ | ❌ |

  
### Response Delivery
  
| Streaming | Structured Output | JSON Mode | Function Call | Text Format |
|---------|---------|---------|---------|---------|
| ✅ | ✅ | ✅ | ✅ | ✅ |

  
### Advanced Reasoning
  
| Basic Reasoning | Reasoning Effort | Reasoning Tokens | Include Reasoning | Verbosity Control |
|---------|---------|---------|---------|---------|
| ✅ | ❌ | ❌ | ❌ | ❌ |

  
## 🎛️ Generation Controls
  
### Architecture Details
  
| Parameter Count | Architecture Type | Tokenizer | Quantization | Fine-Tuned | Base Model |
|---------|---------|---------|---------|---------|---------|
| 236B | transformer | deepseek | None | Yes | deepseek-coder-v2 |

  
### Model Tags
  
| Coding | Writing | Reasoning | Math | Chat | Multimodal | Function Calling |
|---------|---------|---------|---------|---------|---------|---------|
| ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |

  
  
**Additional Tags**
: instruct
  
### Sampling & Decoding
  
| Temperature | Top-P |
|---------|---------|
| 0.0-2.0 | 0.0-1.0 |

  
### Length & Termination
  
| Max Tokens | Stop Sequences |
|---------|---------|
| 1-8k | ✅ |

  
### Repetition Control
  
| Frequency Penalty | Presence Penalty | Repetition Penalty |
|---------|---------|---------|
| -2.0 to 2.0 | -2.0 to 2.0 | ✅ |

  
### Advanced Controls
  
| Deterministic Seed |
|---------|
| ✅ |

  
## 💰 Pricing
  
*Pricing shown for DeepSeek*
  
  
Contact provider for pricing information.
  
## 🚀 Advanced Features
  
### Tool Configuration
  
**Supported Tool Choices**: auto, none, required
  
  
### Response Delivery Options
  
**Response Formats**: text, json
  
**Streaming Modes**: sse
  
**Protocols**: http
  
  
## 📋 Metadata
  
**Created**: 2024-06-20 00:00:00 UTC
  
**Last Updated**: 2024-06-20 00:00:00 UTC
  
  
---
  
  
### Navigation

- [More models by DeepSeek](../)
- [All Providers](../../../../providers)
- [Back to Catalog](../../../..)


---
_Last Updated: 2025-08-31 22:59:24 UTC | Generated by [Starmap](https://github.com/agentstation/starmap)_
