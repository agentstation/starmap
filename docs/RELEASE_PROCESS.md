# Release Process

This document describes the release process for Starmap.

## Prerequisites

1. **GitHub Pages**: Must be enabled in repository settings
   ```bash
   # Enable via GitHub CLI
   gh repo edit --enable-pages
   ```
   Configure source as "GitHub Actions" in Settings → Pages

2. **GoReleaser**: Install for local testing
   ```bash
   # macOS
   brew install goreleaser
   
   # Linux
   snap install goreleaser
   
   # Or download from https://goreleaser.com
   ```

3. **Permissions**: Ensure you have:
   - Write access to the repository
   - Ability to create tags and releases
   - (Optional) HOMEBREW_TAP_TOKEN for Homebrew formula updates

## Release Workflow

### 1. Prepare Release

```bash
# Ensure you're on master branch
git checkout master
git pull origin master

# Run pre-release checks
make release

# This will:
# - Clean the build
# - Format code
# - Tidy modules
# - Run linter
# - Run all tests
```

### 2. Update Version Information

1. Update `CHANGELOG.md`:
   - Move items from "Unreleased" to new version section
   - Add release date
   - Update comparison links

2. Commit changes:
   ```bash
   git add CHANGELOG.md
   git commit -m "chore: prepare for v0.1.0 release"
   git push origin master
   ```

### 3. Create Release Tag

```bash
# Create and push release tag
make release-tag VERSION=0.1.0

# This will:
# - Create annotated tag v0.1.0
# - Push tag to GitHub
# - Trigger release workflow
```

### 4. Monitor Release

The GitHub Actions workflow will automatically:
1. Run tests
2. Build binaries for multiple platforms (Linux, macOS, Windows)
3. Create Docker images and push to ghcr.io
4. Generate release notes from commit messages
5. Create GitHub release with artifacts
6. Update Homebrew tap (if token configured)
7. Deploy documentation to GitHub Pages

Monitor progress at: https://github.com/agentstation/starmap/actions

### 5. Verify Release

After the workflow completes:

1. **Check GitHub Release**: https://github.com/agentstation/starmap/releases
2. **Verify Docker images**: 
   ```bash
   docker pull ghcr.io/agentstation/starmap:latest
   ```
3. **Test installation methods**:
   ```bash
   # Go install
   go install github.com/agentstation/starmap/cmd/starmap@latest
   
   # Homebrew (if configured)
   brew tap agentstation/tap
   brew install starmap
   ```
4. **Check documentation**: Visit https://starmap.agentstation.ai/

## Local Testing

### Test Release Build Locally

```bash
# Create snapshot release (no tag required)
make release-snapshot

# Or with existing tag
make release-local

# Check output in ./dist/
ls -la dist/
```

### Test Documentation Build

```bash
# Generate documentation
starmap generate docs

# Build Hugo site
starmap generate site

# Serve locally
cd site && hugo serve
```

## Versioning Strategy

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (x.0.0): Breaking API changes
- **MINOR** (0.x.0): New features, backward compatible
- **PATCH** (0.0.x): Bug fixes, backward compatible

### Version Numbering Examples

- `v0.1.0`: Initial release
- `v0.2.0`: New provider added
- `v0.2.1`: Bug fix in provider client
- `v1.0.0`: Stable API, production ready
- `v2.0.0`: Major redesign with breaking changes

## Troubleshooting

### Release Workflow Fails

1. Check GitHub Actions logs
2. Ensure all tests pass: `make test`
3. Verify GoReleaser config: `goreleaser check`

### Docker Build Issues

1. Ensure Dockerfile exists and is valid
2. Check GitHub Container Registry permissions
3. Verify login credentials in workflow

### Documentation Not Deploying

1. Ensure GitHub Pages is enabled
2. Check Hugo workflow succeeded
3. Verify site builds locally: `cd site && hugo`

### Homebrew Tap Not Updating

1. Ensure HOMEBREW_TAP_TOKEN secret is set
2. Verify tap repository exists: agentstation/homebrew-tap
3. Check formula generation in goreleaser logs

## Release Checklist

- [ ] All tests passing (`make test`)
- [ ] Code formatted (`make fmt`)
- [ ] Linting clean (`make lint`)
- [ ] CHANGELOG.md updated
- [ ] Documentation updated if needed
- [ ] Version tag follows semver (vX.Y.Z)
- [ ] Release notes reviewed
- [ ] Installation methods tested
- [ ] Documentation site accessible

## Emergency Rollback

If a release has critical issues:

1. **Delete release** (keep tag for history):
   ```bash
   gh release delete vX.Y.Z --yes
   ```

2. **Create patch release** with fix:
   ```bash
   # Fix the issue
   git add .
   git commit -m "fix: critical issue in vX.Y.Z"
   git push origin master
   
   # Create new patch version
   make release-tag VERSION=X.Y.Z+1
   ```

3. **Notify users** via:
   - GitHub Discussions
   - Release notes
   - Project README update

## Continuous Improvement

After each release:
1. Review what went well
2. Note any issues encountered
3. Update this document with learnings
4. Improve automation where possible