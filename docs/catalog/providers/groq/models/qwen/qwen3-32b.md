# qwen3-32b

## Overview 📋

- **ID**: `qwen/qwen3-32b`
- **Provider**: <img src="../logo.svg" alt="" width="20" height="20" style="vertical-align: middle"> [Groq](../README.md)
- **Authors**: [Alibaba Cloud](../../../../authors/alibaba cloud/README.md)
- **Release Date**: 2024-12-23
- **Knowledge Cutoff**: 2024-11-08
- **Open Weights**: true
- **Context Window**: 131.1K tokens
- **Max Output**: 16.4K tokens

## Capabilities 🎯

### Input/Output Modalities

| Direction | Text | Image | Audio | Video | PDF |
|-----------|------|-------|-------|-------|-----|
| Input     | ✅   | ❌   | ❌   | ❌   | ❌   |
| Output    | ✅   | ❌   | ❌   | ❌   | ❌   |

### Core Features

| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |
|--------------|------------------|-------------|------------|------------------|
| ❌           | ❌               | ❌          | ❌         | ❌               |

### Response Delivery

| Streaming | Structured Output | JSON Mode | Function Call | Text Format |
|-----------|-------------------|-----------|---------------|--------------|
| ✅        | ❌                | ❌        | ❌            | ✅           |

## Technical Specifications ⚙️

### Generation Controls (Part 1)

| Temperature | Top-P | Top-K | Top-A | Min-P | Max Tokens |
|-------------|-------|-------|-------|-------|------------|
| ✅ (0-2.0)  | ✅ (0-1) | ❌        | ❌        | ❌        | ✅ (1-16.4K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ✅ (-2 to 2)      | ✅ (-2 to 2)     | ❌                 | ❌         | ✅   | ✅             | ❌        |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $0.29/1M | $0.59/1M | - | - | - |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Groq](../../README.md)
- [← Back to Providers](../../../README.md)
- [← Back to Main Index](../../../../README.md)
