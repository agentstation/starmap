# gpt-4.1

## Overview 📋

- **ID**: `gpt-4.1`
- **Provider**: [OpenAI](../README.md)
- **Authors**: [OpenAI](../../../authors/openai/README.md)
- **Release Date**: 2025-04-14
- **Knowledge Cutoff**: 2024-04-01
- **Open Weights**: false
- **Context Window**: 1.0M tokens
- **Max Output**: 32.8K tokens

## Capabilities 🎯

### Input/Output Modalities

| Direction | Text | Image | Audio | Video | PDF |
|-----------|------|-------|-------|-------|-----|
| Input     | ✅   | ❌   | ❌   | ❌   | ❌   |
| Output    | ✅   | ❌   | ❌   | ❌   | ❌   |

### Core Features

| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |
|--------------|------------------|-------------|------------|------------------|
| ❌           | ✅               | ✅          | ❌         | ❌               |

### Response Delivery

| Streaming | Structured Output | JSON Mode | Function Call | Text Format |
|-----------|-------------------|-----------|---------------|--------------|
| ✅        | ✅                | ✅        | ❌            | ✅           |

## Technical Specifications ⚙️

### Generation Controls (Part 1)

| Temperature | Top-P | Top-K | Top-A | Min-P | Max Tokens |
|-------------|-------|-------|-------|-------|------------|
| ✅ (0-2.0)  | ✅ (0-1) | ❌        | ❌        | ❌        | ✅ (1-32.8K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ✅ (-2 to 2)      | ✅ (-2 to 2)     | ❌                 | ❌         | ❌   | ✅             | ✅ (0-20) |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $2.00/1M | $8.00/1M | - | $0.50/1M | - |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to OpenAI](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
