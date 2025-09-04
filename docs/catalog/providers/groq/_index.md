# <img src="https://raw.githubusercontent.com/agentstation/starmap/master/internal/embedded/catalog/providers/groq/logo.svg" alt="Groq logo" width="48" height="48" style="vertical-align: middle;"> Groq
  
Ultra-fast inference with custom LPU hardware, offering low-latency model serving.
  
  
## Provider Information
  
| Field | Value |
|---------|---------|
| **Provider ID** | `groq` |
| **Total Models** | 27 |
| **Authentication** | API Key Required |
| **Environment Variable** | `GROQ_API_KEY` |
| **Status Page** | [https://status.groq.com](https://status.groq.com) |

  
## 🔗 API Endpoints
  
**Documentation**: [https://console.groq.com/docs/models](https://console.groq.com/docs/models)  
  
**Models API**: [https://api.groq.com/openai/v1/models](https://api.groq.com/openai/v1/models)  
  
**Chat Completions**: [https://api.groq.com/openai/v1/chat/completions](https://api.groq.com/openai/v1/chat/completions)  
  
**Health API**: [https://groqstatus.com/api/v2/summary.json](https://groqstatus.com/api/v2/summary.json)  
  
  
## 🔒 Privacy & Data Handling
  
**Privacy Policy**: [https://groq.com/privacy-policy/](https://groq.com/privacy-policy/)  
  
**Terms of Service**: [https://groq.com/terms-of-use/](https://groq.com/terms-of-use/)  
  
**Retains User Data**: No  
  
**Trains on User Data**: No  
  
  
## ⏱️ Data Retention Policy
  
**Policy Type**: No Retention  
  
**Retention Duration**: Immediate deletion  
  
**Details**: Input prompts and context are not retained; data is processed for immediate response generation and then discarded  
  
  
## 🛡️ Content Moderation
  
**Requires Moderation**: No  
  
**Content Moderated**: Yes  
  
**Moderated by**: Groq  
  
  
## 🏢 Headquarters
  
Mountain View, CA, USA
  
  
## Available Models
  
### GPT
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [gpt-oss-120b](./models/openai-gpt-oss-120b.md) | 131.1k | $0.15 | $0.75 | 📝 ⚡ |
| [gpt-oss-20b](./models/openai-gpt-oss-20b.md) | 131.1k | $0.10 | $0.50 | 📝 ⚡ |

  
### Gemma
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [gemma2-9b-it](./models/gemma2-9b-it.md) | 8.2k | $0.20 | $0.20 | 📝 🔧 ⚡ |

  
### Llama
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [Llama 3 70B](./models/llama3-70b-8192.md) | 8.2k | $0.59 | $0.79 | — |
| [Llama 3 8B](./models/llama3-8b-8192.md) | 8.2k | $0.05 | $0.08 | — |
| [Llama Guard 3 8B](./models/llama-guard-3-8b.md) | 8.2k | $0.20 | $0.20 | — |
| [deepseek-r1-distill-llama-70b](./models/deepseek-r1-distill-llama-70b.md) | 131.1k | $0.75 | $0.99 | 📝 🔧 ⚡ |
| [llama-3.1-8b-instant](./models/llama-3.1-8b-instant.md) | 131.1k | $0.05 | $0.08 | 📝 🔧 ⚡ |
| [llama-3.3-70b-versatile](./models/llama-3.3-70b-versatile.md) | 131.1k | $0.59 | $0.79 | 📝 🔧 ⚡ |
| [llama-4-maverick-17b-128e-instruct](./models/meta-llama-llama-4-maverick-17b-128e-instruct.md) | 131.1k | $0.20 | $0.60 | 📝 🔧 ⚡ |
| [llama-4-scout-17b-16e-instruct](./models/meta-llama-llama-4-scout-17b-16e-instruct.md) | 131.1k | $0.11 | $0.34 | 📝 🔧 ⚡ |
| [llama-guard-4-12b](./models/meta-llama-llama-guard-4-12b.md) | 131.1k | $0.20 | $0.20 | 📝 ⚡ |
| [llama-prompt-guard-2-22m](./models/meta-llama-llama-prompt-guard-2-22m.md) | 512 | N/A | N/A | 📝 ⚡ |
| [llama-prompt-guard-2-86m](./models/meta-llama-llama-prompt-guard-2-86m.md) | 512 | N/A | N/A | 📝 ⚡ |

  
### Mistral
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [Mistral Saba 24B](./models/mistral-saba-24b.md) | 32.8k | $0.79 | $0.79 | — |

  
### Other
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [allam-2-7b](./models/allam-2-7b.md) | 4.1k | N/A | N/A | 📝 ⚡ |
| [compound](./models/groq-compound.md) | 131.1k | N/A | N/A | 📝 ⚡ |
| [compound-beta](./models/compound-beta.md) | 131.1k | N/A | N/A | 📝 ⚡ |
| [compound-beta-mini](./models/compound-beta-mini.md) | 131.1k | N/A | N/A | 📝 ⚡ |
| [compound-mini](./models/groq-compound-mini.md) | 131.1k | N/A | N/A | 📝 ⚡ |
| [kimi-k2-instruct](./models/moonshotai-kimi-k2-instruct.md) | 131.1k | $1.00 | $3.00 | 📝 ⚡ |
| [playai-tts](./models/playai-tts.md) | 8.2k | N/A | N/A | 📝 ⚡ |
| [playai-tts-arabic](./models/playai-tts-arabic.md) | 8.2k | N/A | N/A | 📝 ⚡ |

  
### Qwen
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [Qwen QwQ 32B](./models/qwen-qwq-32b.md) | 131.1k | $0.29 | $0.39 | — |
| [qwen3-32b](./models/qwen-qwen3-32b.md) | 131.1k | $0.29 | $0.59 | 📝 ⚡ |

  
### Whisper
  
| Model | Context | Input | Output | Features |
|---------|---------|---------|---------|---------|
| [whisper-large-v3](./models/whisper-large-v3.md) | 448 | N/A | N/A | 📝 🎵 ⚡ |
| [whisper-large-v3-turbo](./models/whisper-large-v3-turbo.md) | 448 | $0.00 | $0.00 | 📝 🎵 ⚡ |

  
## Configuration
  
### Authentication
  
This provider requires an API key. Set it as an environment variable:
  
  
```bash
export GROQ_API_KEY="your-api-key-here"
```
  
### Using with Starmap
  
```bash
# List all models from this provider
starmap list models --provider groq

# Fetch latest models from provider API
starmap fetch --provider groq

# Sync provider data
starmap sync --provider groq
```
  
### See Also

- [All Providers](../)
- [Browse by Author](../../authors/)
- [Model Comparison](../../models/)


  
---
_[← Back to Providers](../) | [← Back to Catalog](../../) | Generated by [Starmap](https://github.com/agentstation/starmap)_
