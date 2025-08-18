# gpt-3.5-turbo

## Overview 📋

- **ID**: `gpt-3.5-turbo`
- **Author**: [OpenAI](../README.md)
- **Release Date**: 2023-03-01
- **Knowledge Cutoff**: 2021-09-01
- **Open Weights**: false
- **Context Window**: 16.4K tokens
- **Max Output**: 4.1K tokens

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
| ✅ (0-2.0)  | ✅ (0-1) | ❌        | ❌        | ❌        | ✅ (1-4.1K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ✅ (-2 to 2)      | ✅ (-2 to 2)     | ❌                 | ❌         | ❌   | ✅             | ✅ (0-20) |

## Pricing 💰

### Token Pricing

| Input | Output | Reasoning | Cache Read | Cache Write |
|-------|--------|-----------|------------|-------------|
| $0.50/1M | $1.50/1M | - | $1.25/1M | - |

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to OpenAI](../README.md)
- [← Back to Authors](../../README.md)
- [📋 Browse by Provider](../../../providers/README.md)
- [← Back to Main Catalog](../../../README.md)
