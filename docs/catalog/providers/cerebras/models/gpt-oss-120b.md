# gpt-oss-120b

## Overview 📋

- **ID**: `gpt-oss-120b`
- **Provider**: [Cerebras](../README.md)
- **Authors**: [Cerebras](../../../authors/cerebras/README.md)
- **Release Date**: 2025-08-05
- **Open Weights**: true
- **Context Window**: 131.1K tokens
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
| ❌           | ❌               | ❌          | ❌         | ❌               |

### Response Delivery

| Streaming | Structured Output | JSON Mode | Function Call | Text Format |
|-----------|-------------------|-----------|---------------|--------------|
| ✅        | ❌                | ❌        | ❌            | ✅           |

## Technical Specifications ⚙️

### Generation Controls (Part 1)

| Temperature | Top-P | Top-K | Top-A | Min-P | Max Tokens |
|-------------|-------|-------|-------|-------|------------|
| ✅ (0-2.0)  | ✅ (0-1) | ❌        | ❌        | ❌        | ✅ (1-32.8K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ✅ (-2 to 2)      | ✅ (-2 to 2)     | ❌                 | ❌         | ❌   | ✅             | ❌        |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $0.25/1M | $0.69/1M | - | - | - |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Cerebras](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
