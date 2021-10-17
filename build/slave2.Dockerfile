FROM python:3.8-slim-buster

WORKDIR /app

#COPY ./python/slave2/requirements.txt requirements.txt
#RUN pip3 install -r requirements.txt

COPY ./python/slave2/secondary_server.py .

CMD [ "python3", "secondary_server.py"]
