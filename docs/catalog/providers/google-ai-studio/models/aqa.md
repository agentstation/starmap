# Model that performs Attributed Question Answering.

Model trained to return answers to questions that are grounded in provided sources, along with estimating answerable probability.


## Overview 📋

- **ID**: `aqa`
- **Provider**: [Google AI Studio](../README.md)
- **Authors**: [Google](../../../authors/google/README.md)
- **Context Window**: 7.2K tokens
- **Max Output**: 1.0K tokens

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
| ✅ (0-2.0)  | ✅ (0-1) | ✅        | ❌        | ❌        | ✅ (1-1.0K) |

### Generation Controls (Part 2)

| Frequency Penalty | Presence Penalty | Repetition Penalty | Logit Bias | Seed | Stop Sequences | Logprobs |
|-------------------|------------------|--------------------|------------|------|----------------|----------|
| ❌                | ❌               | ❌                 | ❌         | ❌   | ✅             | ❌        |

## Pricing 💰

Contact provider for pricing information.

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Google AI Studio](../README.md)
- [← Back to Providers](../../README.md)
- [← Back to Main Index](../../../README.md)
