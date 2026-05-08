package main

import (
	"bytes"
	"net/http"
	"time"
)

// Qualquer codigo não explicado aqui esta nos outros arquivos. Aqui esta somente a logica do intervalo e o for que é diferente que os outros arquivos.
func main() {
	url := "http://alvo_gateway:8080/post"
	corpo := []byte(`{"msg": "ataque_programado"}`)
	client := &http.Client{}

	totalRequests := 400
	requestsPerSecond := 20

	// Para dizer ao Go que deve ser 20 req por segundo, devemos dividir o segundo time.Second ele é uma unidade de medida fixa que vale exatamente 1 segundo (ou 1 bilhão de nanossegundos).
	// time.Duration(20), dizemos: "Go, transforme esse número 20 em uma unidade de tempo também".
	// Assim discobrimos o intervalo a ser enviado a mensagem pra dar 20 req por segundo.
	interval := time.Second / time.Duration(requestsPerSecond)

	// No Go (e na maioria das linguagens C-like), o for clássico é dividido em três partes distintas que o processador executa em momentos diferentes.
	// A Anatomia do for: sintaxe for i := 0; i < totalRequests; i++ funciona seguindo este fluxo lógico:
	// Inicialização (i := 0): Executada apenas uma vez, no momento em que o código chega no laço. É aqui que a variável é criada na memória RAM.
	// Condição (i < totalRequests): Verificada antes de cada repetição. Se for verdadeira, o código entra no bloco { }. Se for falsa, o laço encerra.
	// Pós-execução (i++): Executada depois que o bloco de código termina, mas antes da próxima verificação. É aqui que o "contador" sobe.
	// Porque o i não reseta a cada repetição? Segue explicação:
	// Rodada 0: Ele cria i com valor 0. Verifica: 0 é menor que 400? Sim. Faz o POST, dorme 50ms. No fim, ele roda o i++. Agora i vale 1.
	// Rodada 1: Ele pula a parte do i := 0 (já foi feita). Verifica: 1 é menor que 400? Sim. Faz o POST, dorme. Roda i++. Agora i vale 2.
	// Agora porque não colocamos o i fora do for? Pois ele deve morrer no momento que a condição i < totalRequests for atingida, depende da estratégia de arquitetura é pra ser economico.
	// Se o i for definido fora, o for perde sua 'memória de nascimento' interna e passa a ser apenas um validador de condição (como um while).
	for i := 0; i < totalRequests; i++ {
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(corpo))
		req.Header.Set("X-Signature", "invalid_key_test")

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}

		time.Sleep(interval) // intervalo
	}
	
	println("Ataque de teste finalizado: 400 disparos efetuados.")
}