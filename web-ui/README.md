# Brockley Web UI

React-based visual workflow builder for Brockley. Provides a drag-and-drop graph editor, execution monitoring, and configuration management.

## Development Setup

```bash
cd web-ui
npm install
npm run dev
```

The dev server starts at [http://localhost:3000](http://localhost:3000) with hot module replacement.

## Build

```bash
npm run build
```

Output goes to `dist/`. The production build is a static bundle served by the Brockley server.

## Configuration

| Variable | Description | Default |
|---|---|---|
| `BROCKLEY_API_URL` | Backend API base URL | `http://localhost:8080/api/v1` |

Set environment variables in a `.env` file in the `web-ui/` directory or pass them at build time.

## Design System

See `docs/design-system/` for component guidelines, color palette, typography, and spacing conventions.

## Tech Stack

- React
- TypeScript
- Vite
- Tailwind CSS
