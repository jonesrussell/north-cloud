FROM python:3.11-slim

WORKDIR /app

# Install curl for healthcheck
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Build placeholder models if not present
RUN python train_and_export.py 2>/dev/null || true

EXPOSE 8077

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8077"]
