# Changelog

All notable changes to Starmap will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Starmap AI Model Catalog System
- Command-line interface for model discovery and comparison
- Support for multiple AI providers (OpenAI, Anthropic, Google, Groq, etc.)
- Embedded catalog with 500+ AI models
- Real-time synchronization with provider APIs
- Comprehensive documentation generation with Hugo
- Markdown-based documentation using github.com/nao1215/markdown
- Multi-source reconciliation engine
- Provider API client implementations
- Model comparison and filtering capabilities
- Pricing and capability information
- Export functionality (OpenAI/OpenRouter formats)

### Infrastructure
- GitHub Actions workflow for Hugo documentation
- GoReleaser configuration for multi-platform releases
- Docker support with automated image builds
- Homebrew tap for macOS/Linux installation
- Test coverage at 93.9% for documentation package

## [0.1.0] - TBD

Initial public release. See Unreleased section for features.

[Unreleased]: https://github.com/agentstation/starmap/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/agentstation/starmap/releases/tag/v0.1.0