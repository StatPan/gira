# GitHub Pages Documentation Site

Gira's public documentation site is built from `docs-site/` and deployed by `.github/workflows/pages.yml`.

## Build

```bash
sh scripts/build-docs-site.sh docs-site site
```

The build copies static site files into `site/` and verifies the required public pages exist.

## Deployment

Pull requests run the docs build. Pushes to `main` upload the generated `site/` artifact and deploy it with GitHub Pages.

## Custom Domain

The Pages artifact includes `CNAME` with:

```text
gira.statpan.com
```

DNS must be configured outside this repository. Point `gira.statpan.com` to GitHub Pages using GitHub's current custom domain guidance, then verify the domain in the repository Pages settings.
