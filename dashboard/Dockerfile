# --- Build Stage ---
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm install

# Copy source
COPY . .

# Build production app
# Note: Ensure NEXT_PUBLIC variables are passed if baked in during build
RUN npm run build

# --- production Stage ---
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV production

# Copy only public and built files
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

EXPOSE 3000

CMD ["npm", "start"]
