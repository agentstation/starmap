# Claude Haiku 3

## Overview 📋

- **ID**: `claude-3-haiku-20240307`
- **Provider**: [Anthropic](../README.md)
- **Authors**: [Anthropic](../../../authors/anthropic/README.md)
- **Release Date**: 2024-03-13
- **Knowledge Cutoff**: 2023-08-31
- **Open Weights**: false
- **Context Window**: 200.0K tokens
- **Max Output**: 4.1K tokens

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
| ✅ (0-2.0)  | ✅ (0-1) | ✅        | ❌        | ❌        | ✅ (1-4.1K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ❌                | ❌               | ❌                 | ❌         | ❌   | ✅             | ❌        |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $0.25/1M | $1.25/1M | - | $0.03/1M | $0.30/1M |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Anthropic](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
