# Desafio da Pós Go Expert

## Cep to Clima + Open Telemetry

Serviço A: weather-man

Serviço B: weather-api-wrapper

Resumidamente, o cliente chama o serviço weather-man que por sua vez chama o serviço weather-api-wrapper que retorna a temperatura de um determinado CEP.

## Como rodar:

- Crie um .env dentro do serviço weather-api-wrapper com a chave WEATHER_API_KEY=exemplo
- Rode o comando `docker-compose up -d`
- O serviço weather-man estará disponível em http://localhost:8080. Exemplo de request na raíz.
- O Zipkin está disponível em http://localhost:9411.

Exemplo Zipkin:


