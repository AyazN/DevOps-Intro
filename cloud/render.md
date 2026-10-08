# Render Service Configuration

- **Source:** Existing image
- **Image:** ghcr.io/ayazn/devops-intro/quicknotes:v0.1.0
- **Region:** Frankfurt
- **Instance type:** Free
- **Health check path:** /health
- **Environment variables:**
  - PORT=8080
  - ADDR=:8080
- **Public URL:** https://quicknotes-ayazn.onrender.com

## Why existing image (not Git repo build)
Render pulls the exact digest CI already built and Lab 9 scanned — no rebuild
drift, no duplicated cache, faster deploys. The Git-repo option would ship an
artifact that was never scanned.