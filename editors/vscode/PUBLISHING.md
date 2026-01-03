# Publishing the VS Code Extension

This guide explains how to publish the Temper VS Code extension to the marketplace.

## Prerequisites

1. **Azure DevOps Account** - Create one at https://dev.azure.com
2. **Publisher Account** - Create at https://marketplace.visualstudio.com/manage
3. **Personal Access Token (PAT)** - For automated publishing

## One-Time Setup

### 1. Create a Publisher

1. Go to https://marketplace.visualstudio.com/manage
2. Click "Create Publisher"
3. Use publisher ID: `felixgeelhaar` (must match `package.json`)
4. Fill in display name, description, etc.

### 2. Create a Personal Access Token

1. Go to https://dev.azure.com
2. Click your profile icon → "Personal Access Tokens"
3. Click "New Token"
4. Configure:
   - **Name**: `vsce-publish`
   - **Organization**: All accessible organizations
   - **Expiration**: Custom (1 year recommended)
   - **Scopes**: Custom defined → Marketplace → Check "Manage"
5. Copy the token (you won't see it again!)

### 3. Add Secret to GitHub

1. Go to https://github.com/felixgeelhaar/temper/settings/secrets/actions
2. Click "New repository secret"
3. Name: `VSCE_PAT`
4. Value: Your Personal Access Token
5. Click "Add secret"

## Publishing

### Automated (Recommended)

Publishing happens automatically when you push a version tag:

```bash
# Update version in CHANGELOG.md
# Then tag and push
git tag vscode-v0.1.0
git push origin vscode-v0.1.0
```

The GitHub Action will:
1. Build and test the extension
2. Package as VSIX
3. Create a GitHub Release with the VSIX
4. Publish to VS Code Marketplace (if `VSCE_PAT` is set)

### Manual Publishing

```bash
cd editors/vscode

# Install vsce
npm install -g @vscode/vsce

# Login (first time only)
vsce login felixgeelhaar

# Package
npm run compile
vsce package

# Publish
vsce publish
```

## Version Bump Checklist

Before releasing a new version:

1. [ ] Update `CHANGELOG.md` with changes
2. [ ] Test the extension locally (F5 in VS Code)
3. [ ] Verify all commands work
4. [ ] Update version in `package.json` (if manual)
5. [ ] Create and push tag

## Troubleshooting

### "Publisher not found"

Ensure the publisher ID in `package.json` matches your marketplace publisher exactly.

### "Access denied"

Your PAT may have expired or lack the Marketplace scope. Create a new one.

### "Missing required files"

Ensure `icon.png` exists and is 128x128 pixels minimum.

### Verify before publish

```bash
vsce ls  # List files that will be included
vsce package --out test.vsix  # Test packaging
```

## Extension Metadata

The extension metadata comes from `package.json`:

| Field | Purpose |
|-------|---------|
| `name` | Unique identifier |
| `displayName` | Shown in marketplace |
| `description` | Short description |
| `publisher` | Your publisher ID |
| `icon` | 128x128+ PNG |
| `categories` | Marketplace categories |
| `keywords` | Search terms |
| `repository` | Source code link |

## Resources

- [vsce CLI](https://github.com/microsoft/vscode-vsce)
- [Publishing Extensions](https://code.visualstudio.com/api/working-with-extensions/publishing-extension)
- [Marketplace Guidelines](https://code.visualstudio.com/api/references/extension-guidelines)
