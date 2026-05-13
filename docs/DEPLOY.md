# Documentation Deployment Guide

This guide explains how to build and deploy the 0G Signal Intelligence Network documentation.

## Options for Hosting

### Option 1: Docsify (Recommended - Easiest)

Docsify renders markdown files on-the-fly in the browser, requiring no build step.

**Install**:
```bash
npm install -g docsify-cli
```

**Local Development**:
```bash
cd docs
docsify serve .
```

Access at: `http://localhost:3000`

**Deploy to GitHub Pages**:
1. Push `docs/` folder to GitHub
2. Enable GitHub Pages in repository settings
3. Set source to `main` branch, `/docs` folder
4. Done! Docs will be live at `https://yourusername.github.io/repo-name/`

**Deploy to Vercel**:
```bash
# Install Vercel CLI
npm install -g vercel

# Deploy
cd docs
vercel --prod
```

### Option 2: GitBook

GitBook provides a polished documentation experience with built-in search, versioning, and hosting.

**Setup**:
1. Go to [gitbook.com](https://www.gitbook.com)
2. Create new space
3. Connect to GitHub repository
4. Point to `docs/` directory
5. GitBook will auto-build and host

**Sync with GitHub**:
GitBook can auto-sync with your GitHub repo, updating docs on every push.

### Option 3: Static Site (VitePress/Docusaurus)

For more customization and features.

**VitePress** (Vue-based):
```bash
npm install -D vitepress
npx vitepress init
```

**Docusaurus** (React-based):
```bash
npx create-docusaurus@latest my-website classic
```

Both support MDX (Markdown + JSX) for interactive components.

### Option 4: Self-Hosted with Nginx

**Build static site**:
```bash
npm install -g gitbook-cli
cd docs
gitbook build
```

**Deploy with Nginx**:
```nginx
server {
    listen 80;
    server_name docs.0g-signals.network;

    root /var/www/docs/_book;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

---

## Quick Deploy Commands

### GitHub Pages
```bash
# One-time setup
git subtree push --prefix docs origin gh-pages

# Or use GitHub Actions (automatic)
# See .github/workflows/deploy-docs.yml below
```

### Vercel
```bash
cd docs
vercel --prod
```

### Netlify
```bash
# Install Netlify CLI
npm install -g netlify-cli

# Deploy
cd docs
netlify deploy --prod --dir=.
```

---

## GitHub Actions Auto-Deploy

Create `.github/workflows/deploy-docs.yml`:

```yaml
name: Deploy Documentation

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Deploy to GitHub Pages
        uses: peaceiris/actions-gh-pages@v3
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./docs
          publish_branch: gh-pages
```

This automatically deploys to GitHub Pages whenever you push changes to `docs/`.

---

## Custom Domain Setup

### GitHub Pages

1. Add `CNAME` file to `docs/` with your domain:
   ```
   docs.0g-signals.network
   ```

2. Configure DNS:
   ```
   A Record:  docs.0g-signals.network → 185.199.108.153
   A Record:  docs.0g-signals.network → 185.199.109.153
   A Record:  docs.0g-signals.network → 185.199.110.153
   A Record:  docs.0g-signals.network → 185.199.111.153
   ```

3. Enable HTTPS in GitHub Pages settings

### Vercel

1. Deploy with Vercel CLI
2. Add domain in Vercel dashboard
3. Configure DNS (Vercel provides instructions)
4. SSL configured automatically

---

## Documentation Structure

```
docs/
├── README.md              # Homepage
├── SUMMARY.md            # Navigation (GitBook)
├── index.html            # Docsify entry point
├── book.json             # GitBook config
├── package.json          # NPM scripts
│
├── api/                  # API Reference
│   ├── rest.md
│   ├── websocket.md
│   └── ...
│
├── providers/            # For Signal Providers
│   ├── getting-started.md
│   └── ...
│
├── consumers/            # For Signal Consumers
│   ├── getting-started.md
│   └── ...
│
├── contracts/            # Smart Contract Docs
│   ├── architecture.md
│   └── ...
│
└── examples/             # Code Examples
    └── ...
```

---

## Updating Documentation

### Adding New Pages

1. Create markdown file in appropriate directory:
   ```bash
   touch docs/concepts/new-concept.md
   ```

2. Add to `SUMMARY.md`:
   ```markdown
   ## Core Concepts
   * [New Concept](concepts/new-concept.md)
   ```

3. Commit and push:
   ```bash
   git add docs/
   git commit -m "docs: add new concept page"
   git push
   ```

### Writing Guidelines

- Use clear, concise language
- Include code examples for technical concepts
- Add diagrams for architecture
- Link to related pages
- Keep each page focused on one topic

### Code Examples

Always include language identifier for syntax highlighting:

````markdown
```python
# Python example
print("Hello")
```

```typescript
// TypeScript example
console.log("Hello");
```

```solidity
// Solidity example
contract Example {}
```
````

---

## Testing Locally

### Test with Docsify
```bash
cd docs
docsify serve .
# Open http://localhost:3000
```

### Test with Python
```bash
cd docs
python -m http.server 8000
# Open http://localhost:8000
```

### Test Links
```bash
# Install markdown-link-check
npm install -g markdown-link-check

# Check all links
find docs -name "*.md" -exec markdown-link-check {} \;
```

---

## SEO Optimization

### Meta Tags

Add to `index.html`:
```html
<meta property="og:title" content="0G Signal Intelligence Network Docs">
<meta property="og:description" content="Build AI trading agents on 0G">
<meta property="og:image" content="https://docs.0g-signals.network/og-image.png">
<meta name="twitter:card" content="summary_large_image">
```

### Sitemap

Generate sitemap for search engines:
```bash
npm install -g sitemap-generator-cli
sitemap-generator https://docs.0g-signals.network -f docs/sitemap.xml
```

### robots.txt

Create `docs/robots.txt`:
```
User-agent: *
Allow: /
Sitemap: https://docs.0g-signals.network/sitemap.xml
```

---

## Analytics

### Google Analytics

Add to `index.html`:
```html
<script async src="https://www.googletagmanager.com/gtag/js?id=GA_MEASUREMENT_ID"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'GA_MEASUREMENT_ID');
</script>
```

### Plausible Analytics

Privacy-friendly alternative:
```html
<script defer data-domain="docs.0g-signals.network" src="https://plausible.io/js/script.js"></script>
```

---

## Maintenance

### Regular Updates

- Review documentation monthly
- Update code examples when API changes
- Fix broken links
- Add new features as they're released
- Respond to community feedback

### Version Management

For breaking changes, maintain multiple versions:

```
docs/
├── v1/
│   └── README.md
├── v2/
│   └── README.md
└── README.md  (latest)
```

---

## Need Help?

- Docsify Docs: https://docsify.js.org
- GitBook Docs: https://docs.gitbook.com
- GitHub Pages: https://pages.github.com
- Vercel: https://vercel.com/docs

---

**Recommended Approach**: Start with Docsify + GitHub Pages for simplicity, migrate to GitBook later if needed.
