FROM node:20-alpine

WORKDIR /app

COPY web/package.json web/pnpm-lock.yaml ./

# Install pnpm (using Corepack is recommended)
RUN corepack enable && corepack prepare pnpm@latest --activate

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy the rest of the application code
# COPY . .

# Expose ports if your dev server uses them (e.g., 3000 for Next.js or Vite)
EXPOSE 3000

# Command to run your development script (e.g., "dev" script in package.json)
CMD ["pnpm", "run", "dev"]
