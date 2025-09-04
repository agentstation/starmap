# Starmap Documentation Site

This directory contains the Hugo-based documentation website for Starmap.

## Structure

```
site/
├── hugo.yaml       # Hugo configuration
├── content/        # Symlink to ../docs/
├── themes/         # Hugo themes (hugo-book)
├── static/         # Static assets (CSS, JS, images)
├── layouts/        # Custom layout overrides
├── assets/         # Processed assets
└── public/         # Generated site (git-ignored)
```

## Quick Start

### Prerequisites

1. **Using devbox (recommended)**:
```bash
# Load all required tools automatically
devbox shell
```

2. **Manual installation**:
```bash
# macOS
brew install hugo

# Linux
snap install hugo --channel=extended

# Windows
choco install hugo-extended
```

### Development Workflow

1. **Generate the site**:
```bash
# Using starmap CLI
starmap site generate

# Or using make
make site-generate
```

2. **Run development server**:
```bash
# Using starmap CLI
starmap site serve

# Or using make
make site-serve

# Visit http://localhost:1313
```

3. **Build for production**:
```bash
# Using make
make site-build

# Or directly with Hugo
cd site && hugo --minify --gc
```

## Deployment

The site is automatically deployed to GitHub Pages via GitHub Actions when:
- Changes are pushed to the master branch
- Version tags are created (v1.0.0, v2.0.0, etc.)

### Manual Deployment

1. Push changes to master branch
2. GitHub Actions workflow runs automatically
3. Site deploys to: https://[username].github.io/starmap/

### Tagged Release Deployment

1. Create a version tag: `git tag -a v1.0.0 -m "Release v1.0.0"`
2. Push the tag: `git push origin v1.0.0`
3. GitHub Actions deploys the versioned documentation

### Preview Deployments

Pull requests automatically generate preview builds that can be downloaded as artifacts from the GitHub Actions run.

## Configuration

### Hugo Configuration

Edit `hugo.yaml` to modify:
- Site title and description
- Base URL
- Theme settings
- Menu structure
- Search configuration

### Content Organization

The `content/` directory is a symlink to `../docs/`, preserving the existing documentation structure:

```
docs/
├── catalog/
│   ├── providers/     # Provider documentation
│   ├── authors/       # Author documentation
│   └── README.md      # Catalog overview
├── PROVIDER_IMPLEMENTATION_GUIDE.md
└── ...
```

### Theme Customization

The site uses the [Hugo Book](https://github.com/alex-shpak/hugo-book) theme, optimized for technical documentation.

To customize:
1. Override layouts in `layouts/`
2. Add custom CSS in `static/css/custom.css`
3. Add custom JS in `static/js/custom.js`

## Features

- **Fast builds**: Sub-second build times for 500+ pages
- **Search**: Built-in search functionality
- **Dark mode**: Automatic light/dark theme
- **Mobile responsive**: Works on all devices
- **Code highlighting**: Syntax highlighting for code blocks
- **Git info**: Shows last modified dates
- **Copy buttons**: Copy code blocks with one click

## Troubleshooting

### Hugo not found

Install Hugo or use devbox:
```bash
devbox shell
```

### Theme not found

The theme should be automatically downloaded. If missing:
```bash
make site-theme
```

### Content not showing

Ensure the content symlink exists:
```bash
cd site
ln -sf ../docs content
```

### Build errors

Check Hugo version (requires 0.110.0+):
```bash
hugo version
```

## Development Tips

### Adding Front Matter

Documentation files can include Hugo front matter for enhanced metadata:

```yaml
---
title: "Page Title"
description: "Page description"
weight: 10
draft: false
---
```

### Custom Shortcodes

Create custom shortcodes in `layouts/shortcodes/` for reusable content components.

### Testing Changes

1. Start the development server: `make site-serve`
2. Edit documentation files in `docs/`
3. Changes are automatically reflected (live reload)
4. Test the production build: `make site-build`

## Advanced Usage

### Multiple Languages

Configure multiple languages in `hugo.yaml` for internationalization support.

### Custom Taxonomies

Add custom taxonomies for better content organization:
- Tags
- Categories
- Providers
- Authors

### Performance Optimization

- Enable asset minification in production
- Use Hugo's image processing for optimization
- Implement lazy loading for images
- Configure CDN for static assets

## Maintenance

### Updating the Theme

```bash
cd site/themes/hugo-book
git pull origin master
```

### Cleaning Generated Files

```bash
# Remove generated files
rm -rf site/public
rm -rf site/resources/_gen

# Or use Hugo's clean command
cd site && hugo --cleanDestinationDir
```

## License

The documentation site uses the same AGPL-3.0 license as the Starmap project.