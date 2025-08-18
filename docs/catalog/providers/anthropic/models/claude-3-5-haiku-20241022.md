# Claude Haiku 3.5

## Overview 📋

- **ID**: `claude-3-5-haiku-20241022`
- **Provider**: [Anthropic](../README.md)
- **Authors**: [Anthropic](../../../authors/anthropic/README.md)
- **Release Date**: 2024-10-22
- **Knowledge Cutoff**: 2024-07-31
- **Open Weights**: false
- **Context Window**: 200.0K tokens
- **Max Output**: 8.2K tokens

## Capabilities 🎯

### Input/Output Modalities

| Direction | Text | Image | Audio | Video | PDF |
|-----------|------|-------|-------|-------|-----|
| Input     | ✅   | ✅   | ❌   | ❌   | ❌   |
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
| ✅ (0-2.0)  | ✅ (0-1) | ✅        | ❌        | ❌        | ✅ (1-8.2K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ❌                | ❌               | ❌                 | ❌         | ❌   | ✅             | ❌        |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $0.80/1M | $4.00/1M | - | $0.08/1M | $1.00/1M |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Anthropic](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
