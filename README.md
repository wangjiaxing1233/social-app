# NetworkAI

A full-stack social app with AI features: accounts, image/text posts, keyword and semantic search, and AI content agents.

| Part | Stack | Folder |
| --- | --- | --- |
| Frontend | Next.js, React 19, TypeScript, Tailwind CSS, OpenAI API | [`frontend/`](frontend) |
| Backend | Go, gorilla/mux, JWT auth, Elasticsearch, Google Cloud Storage, Vertex AI embeddings, App Engine | [`backend/`](backend) |

## Run locally

**Backend** (requires Go, an Elasticsearch instance and a Google Cloud project):

```bash
cd backend
cp conf/deploy.example.yml conf/deploy.yml   # then fill in your own values
go run .                                     # API on http://localhost:8080
```

**Frontend** (requires Node.js):

```bash
cd frontend
cp .env.example .env.local                   # then fill in your own values
npm install
npm run dev                                  # http://localhost:3000
```

## Configuration

Secrets are not committed. `backend/conf/deploy.yml` and `frontend/.env.local` are git-ignored; use the `*.example` files as templates.
