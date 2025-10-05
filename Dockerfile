# Stage 1: Builder
FROM python:3.11-slim as builder

WORKDIR /app

# Copy and install dependencies
COPY api/requirements.txt .
RUN pip wheel --no-cache-dir --wheel-dir /app/wheels -r requirements.txt

# Stage 2: Final
FROM python:3.11-slim

WORKDIR /app

# Copy installed dependencies from builder
COPY --from=builder /app/wheels /wheels
RUN pip install --no-cache /wheels/*

# Copy application code and necessary directories
COPY api/ /app/api/
COPY settings/ /app/settings/
COPY web/build/ /app/web/build/
COPY config.yaml /app/

# Expose the application port
EXPOSE 8080

# Run the application with Gunicorn
CMD ["gunicorn", "--bind", "0.0.0.0:8080", "api.app:app"]
