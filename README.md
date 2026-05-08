# Go High-Performance Gateway 🚀

Gateway de validação criptográfica (HMAC) focado em alta concorrência e baixo consumo de recursos. Tudo comentado para melhor aprendizado. Em geral é um teste BRUTO.

### Performance Alcançada:
- **CPU:** 301.35% (Escalabilidade real em 3 núcleos)
- **RAM:** ~51MiB (Estabilidade via sync.Pool)
- **Arquitetura:** Docker Scratch (imagem ultra-leve)

### Tecnologias:
- Golang 1.23-alpine
- Docker & Docker Compose
- Criptografia HMAC-SHA256
- Concorrência Lock-free (Atomic)

### Como rodar:
\`\`\`bash
docker-compose up --build
\`\`\`
EOF