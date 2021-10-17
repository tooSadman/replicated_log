FROM python:3.8-slim-buster

WORKDIR /app

COPY ./python/slave1/slave1.py .

CMD [ "python3", "slave1.py"]
