# Homebrew-Cask Submission Guide

## Status Tracking

- [ ] Met notability requirements (75+ stars OR 30+ forks/watchers)
- [ ] First manual submission completed
- [ ] Accepted into homebrew-cask
- [ ] Automated updates enabled

## Overview

Starmap is distributed via Homebrew using two methods:

| Method | Command | Status |
|--------|---------|---------|
| **Tap** | `brew install agentstation/tap/starmap` | ✅ Active |
| **Official Cask** | `brew install --cask starmap` | ⏳ Pending submission |

The official cask provides wider discovery and easier installation for macOS users.

## Prerequisites for Submission

### Notability Requirements
Homebrew-cask requires meeting one of these criteria:
- ⭐ **75+ stars** on GitHub, OR
- 🍴 **30+ forks** on GitHub, OR  
- 👀 **30+ watchers** on GitHub

### Release Automation
The release workflow automatically:
1. Checks if we meet notability requirements on each release
2. Creates a submission issue when eligible
3. Points to the generated cask file from our tap

## First-Time Submission Process (Manual)

When the release workflow creates a submission issue:

### 1. Get the Generated Cask File
GoReleaser automatically maintains a properly formatted cask:
```bash
# Download the current cask
curl -O https://raw.githubusercontent.com/agentstation/homebrew-tap/main/Casks/starmap.rb

# Review the contents
cat starmap.rb
```

### 2. Test the Cask Locally
```bash
# Install homebrew-cask if needed
brew tap homebrew/cask

# Test audit and style
brew audit --cask starmap
brew style --cask starmap

# Optional: Test installation
brew install --cask starmap
starmap --version
brew uninstall --cask starmap
```

### 3. Submit to Homebrew-Cask
```bash
# Fork and clone homebrew-cask
gh repo fork homebrew/homebrew-cask --clone
cd homebrew-cask

# Create a branch
git checkout -b add-starmap-vX.X.X

# Add the cask file
cp ../starmap.rb Casks/s/starmap.rb

# Commit
git add Casks/s/starmap.rb
git commit -m "Add starmap vX.X.X"

# Push and create PR
git push origin add-starmap-vX.X.X
gh pr create \
  --title "Add starmap vX.X.X" \
  --body "## New cask: starmap vX.X.X

**Description**: AI Model Catalog System - Discover, compare, and sync AI models across providers

**Project**: https://github.com/agentstation/starmap

**Notability**: 
- ⭐ Stars: XX
- 🍴 Forks: XX
- 👀 Watchers: XX

**Testing**: 
- ✅ Tested on Intel Mac
- ✅ Tested on Apple Silicon Mac  
- ✅ Passes \`brew audit --cask starmap\`
- ✅ Passes \`brew style --cask starmap\`

**Binary Distribution**: 
This is a Go binary-only distribution. The cask includes shell completions and man pages."
```

### 4. Monitor the PR
- Respond to reviewer feedback promptly
- Make requested changes if needed
- Be patient - review can take several days

## After First Acceptance

### 1. Set Up Automated Updates
Once the cask is accepted into homebrew-cask:

```bash
# Create a GitHub Personal Access Token
# Settings → Developer Settings → Personal Access Tokens (Classic)
# Scopes needed: public_repo, workflow
```

Add the token as a repository secret:
- Repository Settings → Secrets and Variables → Actions
- Name: `HOMEBREW_PAT`
- Value: Your PAT token

### 2. Enable Automated Workflow
The `bump-homebrew-cask.yaml` workflow will automatically:
- Detect when starmap exists in homebrew-cask
- Create bump PRs on each release using `brew bump-cask-pr`
- Handle version updates and SHA256 generation

### 3. Update Documentation
After acceptance, update the README to include:
```markdown
### macOS Installation

**Homebrew (Official):**
```bash
brew install --cask starmap
```

**Homebrew (Tap):**
```bash  
brew install agentstation/tap/starmap
```
```

## Current Cask Configuration

Our GoReleaser configuration generates a cask with:

✅ **Multi-architecture support**: Intel and Apple Silicon  
✅ **Shell completions**: Bash, Zsh, Fish  
✅ **Man pages**: Installed to standard locations  
✅ **Quarantine removal**: Handles unsigned binary  
✅ **User-friendly caveats**: Quick start instructions  
✅ **Livecheck**: Automatic version detection  

## Troubleshooting

### Common Issues

**"This cask does not exist"**
- Ensure you're testing with the file in the correct location
- Check that homebrew-cask tap is installed: `brew tap homebrew/cask`

**"SHA256 mismatch"**
- The cask file may be outdated - get the latest from our tap
- GoReleaser updates SHA256 automatically on each release

**"Binary fails to run"** 
- macOS may quarantine the binary - the cask includes quarantine removal
- Try manual removal: `xattr -dr com.apple.quarantine /path/to/starmap`

### Getting Help

- **Homebrew-cask issues**: Review [Contributing Guide](https://github.com/Homebrew/homebrew-cask/blob/master/CONTRIBUTING.md)
- **Starmap issues**: Create issue on [starmap repository](https://github.com/agentstation/starmap/issues)
- **Release automation**: Check GitHub Actions logs

## Benefits After Submission

🌟 **Discoverability**: Listed in official Homebrew search  
⚡ **Easy installation**: No tap needed  
🔄 **Automatic updates**: Via `brew upgrade --cask`  
📊 **Usage tracking**: Homebrew analytics (if enabled)  
🛡️ **Trust**: Official Homebrew security review  