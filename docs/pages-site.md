# GitHub Pages Documentation Site

Gira's public documentation site is built from `docs-site/` with VitePress and deployed by `.github/workflows/pages.yml`.

The docs toolchain is separate from the product runtime. The product remains the Go-built `gira` binary; Node is used only to build static documentation.

## Build

```bash
npm ci
sh scripts/build-docs-site.sh docs-site site
```

The build emits static HTML into `site/` and verifies the required public pages exist.

For local editing:

```bash
npm run docs:dev
```

## Deployment

Pull requests run the docs build. Pushes to `main` upload the generated `site/` artifact and deploy it with GitHub Pages.

GitHub Pages is an enabled, required delivery path for pushes to `main`. Pages configuration or deployment failures fail the corresponding workflow job. The workflow emits an actionable error annotation before stopping; it does not convert a failed Pages operation into a successful run. Pull requests remain build-only and never configure or deploy Pages.

## Custom Domain

The Pages artifact includes `CNAME` from `docs-site/public/CNAME` with:

```text
gira.statpan.com
```

DNS must be configured outside this repository. Point `gira.statpan.com` to GitHub Pages using GitHub's current custom domain guidance, then verify the domain in the repository Pages settings.
