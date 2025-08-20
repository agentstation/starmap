# Model that performs Attributed Question Answering.

Model trained to return answers to questions that are grounded in provided sources, along with estimating answerable probability.


## Overview 📋

- **ID**: `aqa`
- **Author**: [Google](../README.md)
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

## Generation Controls

### Sampling & Decoding

| Temperature | Top-P | Top-K |
|---|---|---|
| 0.0-2.0 | 0.0-1.0 | ✅ |

### Length & Termination

| Max Tokens | Stop Sequences |
|---|---|
| 1-1.0K | ✅ |

## Pricing 💰

Contact provider for pricing information.

## Metadata 📋

**Created**: 0001-01-01 00:00:00 UTC
**Last Updated**: 0001-01-01 00:00:00 UTC

## Navigation

- [← Back to Google](../README.md)
- [← Back to Authors](../../README.md)
- [📋 Browse by Provider](../../../providers/README.md)
- [← Back to Main Catalog](../../../README.md)
