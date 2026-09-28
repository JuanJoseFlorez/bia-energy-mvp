FROM python:3.12-slim
# LITELLM_LOCAL_MODEL_COST_MAP: use the bundled model cost map instead of fetching it at startup
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 LITELLM_LOCAL_MODEL_COST_MAP=True
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY app ./app
RUN useradd --create-home --uid 10001 engine
USER engine
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
