# Deployment Guide for Starmap Documentation

This guide explains how to deploy the Starmap documentation to GitHub Pages.

## Quick Start

### 1. Enable GitHub Pages

1. Go to your repository's **Settings** → **Pages**
2. Under **Build and deployment**, set **Source** to **GitHub Actions**
3. No branch selection is needed (deployment happens via Actions)

### 2. Deploy to GitHub Pages

Once Pages is enabled, simply push to the `master` branch:

```bash
git push origin master
```

The GitHub Actions workflow will automatically:
- Build the documentation
- Generate the Hugo site
- Deploy to GitHub Pages

Your site will be available at: https://agentstation.github.io/starmap/

## Custom Domain Setup (Optional)

### Using starmap.agentstation.ai

If you want to use a custom domain:

1. **Configure DNS** at your domain provider:
   - Add A records pointing to GitHub's IPs:
     - 185.199.108.153
     - 185.199.109.153
     - 185.199.110.153
     - 185.199.111.153
   - OR add a CNAME record pointing to: `agentstation.github.io`

2. **Enable custom domain** in GitHub:
   - Go to Settings → Pages
   - Enter your custom domain: `starmap.agentstation.ai`
   - Check "Enforce HTTPS"

3. **Update Hugo configuration** (already configured):
   - The CNAME file is already created in `site/static/CNAME`
   - Update `site/hugo.yaml` to use your custom domain:
     ```yaml
     baseURL: "https://starmap.agentstation.ai/"
     ```

## Local Testing

### Test with GitHub Pages URL
```bash
make site-test-pages
```
Opens: http://localhost:1313 (preview with GitHub Pages URL)

### Test with Custom Domain
```bash
make site-test-custom
```
Opens: http://localhost:1313 (preview with custom domain URL)

### Check Deployment Readiness
```bash
make deploy-check
```
This verifies all requirements are met for deployment.

## Deployment Workflow

### Automatic Deployment (Production)

Every push to `master` triggers automatic deployment via GitHub Actions:

1. **Build Phase**:
   - Generates documentation from Go code
   - Builds Hugo static site
   - Creates deployment artifacts

2. **Deploy Phase**:
   - Uploads to GitHub Pages
   - Site goes live within minutes

### PR Preview Deployments

Pull requests get automatic preview deployments to Surge.sh:

1. Open a PR
2. Wait for the build to complete
3. Find the preview link in the PR comments
4. Preview URL format: `https://starmap-pr-{number}.surge.sh`

## Project Structure

```
starmap/
├── docs/                 # Generated markdown documentation
│   ├── catalog/         # AI model catalog
│   ├── providers/       # Provider documentation
│   └── authors/         # Author documentation
├── site/                # Hugo site configuration
│   ├── hugo.yaml        # Hugo configuration
│   ├── content/         # Symlink to ../docs
│   ├── static/          # Static assets
│   │   └── CNAME       # Custom domain file
│   ├── themes/          # Hugo theme (hugo-book)
│   └── public/          # Built site (git-ignored)
└── .github/
    └── workflows/
        ├── hugo.yaml    # Main deployment workflow
        └── pr-preview.yaml # PR preview workflow
```

## Troubleshooting

### Pages Not Deploying

1. **Check Pages is enabled**:
   ```
   Settings → Pages → Source: GitHub Actions
   ```

2. **Check workflow status**:
   ```
   Actions tab → Look for hugo.yaml workflow
   ```

3. **Verify deployment readiness**:
   ```bash
   make deploy-check
   ```

### Custom Domain Not Working

1. **Verify DNS records** are propagated (can take up to 48 hours):
   ```bash
   dig starmap.agentstation.ai
   ```

2. **Check CNAME file** exists:
   ```bash
   cat site/static/CNAME
   ```

3. **Ensure HTTPS is enforced** in GitHub Pages settings

### Build Failures

1. **Check Hugo version**:
   ```bash
   hugo version  # Should be 0.148.0+
   ```

2. **Verify symlink**:
   ```bash
   ls -la site/content  # Should point to ../docs
   ```

3. **Update theme**:
   ```bash
   make site-theme
   ```

## Useful Commands

| Command | Description |
|---------|------------|
| `make deploy-check` | Verify deployment readiness |
| `make site-build` | Build production site locally |
| `make site-test-pages` | Test with GitHub Pages URL |
| `make site-test-custom` | Test with custom domain |
| `make catalog-docs` | Regenerate documentation |
| `make site-preview` | Full local preview workflow |

## CI/CD Pipeline

The deployment pipeline consists of:

1. **On Push to Master** (`hugo.yaml`):
   - Builds documentation
   - Deploys to GitHub Pages
   - Updates live site

2. **On Pull Request** (`pr-preview.yaml`):
   - Builds documentation
   - Deploys preview to Surge.sh
   - Comments preview link on PR

3. **Manual Trigger**:
   - Can be triggered from Actions tab
   - Useful for re-deployment without code changes

## Security Considerations

- GitHub Pages serves public content only
- No server-side code execution
- HTTPS enforced by default
- Custom domain requires DNS verification
- Deployment requires GitHub Actions permissions

## Support

For issues or questions:
- Check [GitHub Actions logs](https://github.com/agentstation/starmap/actions)
- Review [GitHub Pages documentation](https://docs.github.com/pages)
- Open an issue in the repository