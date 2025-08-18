# gpt-5

## Overview 📋

- **ID**: `gpt-5`
- **Provider**: [OpenAI](../README.md)
- **Authors**: [OpenAI](../../../authors/openai/README.md)
- **Release Date**: 2025-08-07
- **Knowledge Cutoff**: 2024-09-30
- **Open Weights**: false
- **Context Window**: 400.0K tokens
- **Max Output**: 128.0K tokens

## Capabilities 🎯

### Input/Output Modalities

| Direction | Text | Image | Audio | Video | PDF |
|-----------|------|-------|-------|-------|-----|
| Input     | ✅   | ✅   | ✅   | ❌   | ❌   |
| Output    | ✅   | ❌   | ✅   | ❌   | ❌   |

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
| ✅ (0-2.0)  | ✅ (0-1) | ❌        | ❌        | ❌        | ✅ (1-128.0K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ✅ (-2 to 2)      | ✅ (-2 to 2)     | ❌                 | ❌         | ❌   | ✅             | ❌        |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $1.25/1M | $10.00/1M | - | $0.13/1M | - |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to OpenAI](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
