FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .
COPY add_hosts_windows.bat .
COPY add_hosts_linux_macos.sh .

ENV PYTHONUNBUFFERED=1

CMD ["python", "main.py"]
