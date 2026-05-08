package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var ( // Variaveis definidas, sucesso e falha atomica pra contar certo e evitar polling(concorrencia), as 2 com last não a necessidade, por conta que o sucesso e falha ja é atomicca.
	sucesso     atomic.Uint64
	falha       atomic.Uint64
	lastSucesso uint64
	lastFalha   uint64
)

const hmacSecret = "a4d7014171478639f968070299887ee3c707c17623bbb1e21ba98c3760fe9409" // chave hmac, boa pratica é por variavel e puxar do .env, mas como aqui o foco é teste de performace não ha motivo

// Definimos a variavel global como hmacPool.
// O pacote sync é onde o Go guarda as ferramentas de sincronização. O Pool é uma estrutura que resolve o seguinte problema: criar e destruir coisas o tempo todo gasta muita CPU e memória.
// Em vez de criar um "calculador de HMAC" novo toda vez e jogar no lixo depois, nós guardamos ele. Quando a requisição acaba, devolvemos ele lá. A próxima requisição o pega "usado", limpa e usa de novo, limpeza é feita no logica do func main().
var hmacPool = sync.Pool{

	// New: É o campo que diz: "Se não tiver nenhum objeto sobrando no estoque, execute esta função para fabricar um novo".
	// func() interface{}: não é uma nomeação de algo é dizer algo pro go. Interface no Go é uma instrução para o compilador. Uma instanciação pro go ler e entender que ali é um motor de calculo com aquelas regras. Go, olha através dessa caixa. Diz pro go: 
	// Tá vendo aqueles botões Write e Sum ali dentro? Isso me prova que o que está aí é um motor de cálculo. Agora me deixa usar esses botões.
	// hmac.New: A ferramenta que cria o motor de cálculo.
	// sha256.New: O algoritmo da segurança.
	// []byte(hmacSecret): A chave secreta separada em bytes.
	New: func() interface{} { return hmac.New(sha256.New, []byte(hmacSecret)) },
}

func main() {
	appName := "Gateway"

	// O func em HandleFunc define oque ira acontecer no codigo, é a RECEITA que define a ROTA e junta o 'w' e o 'r' para nomear o http.ResponseWriter(w) e *http.Request(r)
	// [R = O QUE CHEGOU]: Usamos o '*' no r para não xerocar os bytes(endereço na memória RAM), apenas apontar o endereço atravez dos bytes
	// [W = O QUE VAI]: função de responder a quem envia o http
	http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {

		// Usa r de request para buscar assisnatura(r.Header.Get("X-Signature")), o Header é algo de dentro do *http.Request que tira o header ja pronto, é basicamente um Mapa (map[string][]string) que já está preenchido na memória RAM antes mesmo do código começar a rodar. Pemitindo mais de uma assinatura.
		// É como se o Go já tivesse aberto o envelope e colocado os selos e o remetente em uma pastinha organizada.
		// Poderiamos acessar direto via r.Header["X-Signature"], mas o método .Get() é melhor porque:
		// Ignora Maiúsculas/Minúsculas: Se o worker enviar x-signature ou X-Signature, o Go acha do mesmo jeito.
		// Segurança: Ele sempre te devolve uma string limpa, evitando que o código quebre se o header não existir.
		// Nesse caso o Go já usou o io internamente pra puxar dessa forma diferente do bodyBytes.
		sigOriginal := r.Header.Get("X-Signature") // Para entender o resto do codigo precisamos saber que o X-Signature é o resultado do calculo feito no worker, o body do json e o hmac enviado em X-Signature passa pela formula que resulta em um valor especifico que gera a X-Signature.

		// Diferente do Header, o Body (r.Body) não está pronto. Ele é um io.ReadCloser, ou seja, um "fluxo" de dados.
		// Os contatos fictícios ainda estão "viajando" ou guardados no buffer de rede(Quando os dados (o seu JSON) saem do Worker e viajam pela internet, eles não caem direto nas variáveis do seu programa. Eles chegam na sua placa de rede em "pedacinhos" (pacotes).).
		// O corpo da requisição pode ser gigantesco (MBs ou GBs). O Go, por segurança e performance, não lê o corpo automaticamente.
		// O io é o pacote básico do Go para Entrada e Saída (Input/Output). Uma caixa de ferramentas padrão que o Go oferece para lidar com qualquer coisa que transmita dados que venha direto da rede da internet.
		// O ReadAll é uma função específica dentro dessa caixa de ferramentas. O nome já diz tudo: "Leia Tudo" sem filtro(deve ser pensado de maneira estrategica em produção).
		// Quando você usa o io.ReadAll, ele pega esses bytes do buffer e coloca na variavel ele espera acabar e junta tudo.
		bodyBytes, err := io.ReadAll(r.Body)

		// Se err for nil(nada), não deu erro e segue em frente.
		if err != nil {
			// Ele envia o código 500 dentro do envelope da resposta HTTP. Ele não printa no terminal, go é silencioso no padrão.
			w.WriteHeader(http.StatusInternalServerError)
			return // Só ignora
		}

		// Aqui puxamos o objeto, Como o Pool entrega uma caixa fechada (interface{}), usamos o .(hash.Hash) para dizer: "Eu sei que é um motor de hash, libere os botões de comando para mim".
		// A variável h agora é o seu motor pronto para girar.
		h := hmacPool.Get().(hash.Hash)

		// Como esse motor pode ter sido usado por outra requisição milissegundos antes, ele pode estar "sujo" com o resultado do calculo antigo. Damos um Reset para limpar a sujeira.
		h.Reset()

		// É aqui h.Write(bodyBytes) que a mistura do hmac e do body que chegou, literalmente empurramos os dados do JSON para dentro onde ja contem o hmac.
		// O Write é muito foda porque ele não precisa de toda a informação de uma vez só se você não quiser. Você pode dar vários Write seguidos.
		// h.Write: É como colocar os ingredientes no liquidificador e ligar.
		h.Write(bodyBytes) 

		// h.Sum(nil): É quando você desliga o liquidificador(h.Write) e despeja o "suco" no copo. Só aqui você tem a assinatura final. Ele é quem faz o trabalho pesado. Ele pega aquela mistura complexa que aconteceu no h.Write e cospe o resultado final do calculo.
		// hex.EncodeToString: Ele olha para esses números e pensa: "Beleza, os humanos (e o Header HTTP) querem ver isso como texto (Hexadecimal)".
		sigCalculada := hex.EncodeToString(h.Sum(nil))

		// Em vez de deixar o motor ali parado para o lixeiro (Garbage Collector) levar, você joga ele de volta la no Pool(Put). Para reutilizar.
		hmacPool.Put(h)

		// Aqui rola a comparação dos calculos a partir de hmac.Equal. Compara se são iguais.
		// hmac.Equal faz a comparação em tempo constante (evitando que um hacker use um cronômetro para adivinhar a assinatura).
		if hmac.Equal([]byte(sigOriginal), []byte(sigCalculada)) {

			// Se passou ele soma o sucesso na variavel atomic e guarda em current que por sucesso ser atomic ele tambem é.
			current := sucesso.Add(1)
			
			// A cada 150.000 requisições atravez de current, mostramos o JSON no log para conferência
			// Deu 15k ele zera e começa de novo
			if current%150000 == 0 {
				fmt.Printf("\n [AMOSTRA JSON]: %s\n\n", string(bodyBytes))
			}
			
			w.WriteHeader(http.StatusOK) // Retorna sucesso, reconhecido com sucesso!
		} else {
			falha.Add(1) // Se não bateu soma na falha
			w.WriteHeader(http.StatusUnauthorized) // Retorna silenciosamente pro go
		}
	})

	// A Goroutine "Observadora" (go func() { ... }())
	// O go na frente da função cria uma linha de execução independente.
	// Enquanto o seu servidor principal está ocupado processando os 47k req/s, essa função fica "rodando de lado".
	go func() {
		for {
			time.Sleep(time.Second) // A cada segundo ele da um play
			currentSucesso := sucesso.Load() // carrega o sucesso
			currentFalha := falha.Load() // carrega a falha

			// pra calcular quantas req por segundo repare que ali em baixo passamos lastSucesso = currentSucesso assim sempre que passar por aqui ele pega quantas req foram feitas anteriormante menos a atual
			reqPorSegundo := currentSucesso - lastSucesso 

			// da mesma forma que calculamos a req por segundo calculamos a falha como explicado acima
			bloqueiosPorSegundo := currentFalha - lastFalha

			lastSucesso = currentSucesso
			lastFalha = currentFalha
			
			// O que cada símbolo faz:
			// %: "Atenção Go, vem um valor aqui".
			// - (Sinal de menos): Alinha o número à esquerda. Sem o menos, o Go empurra o número para a direita (alinhamento padrão).
			// 6, 4, 8: É a largura mínima. Você está dizendo: "Reserve esse número de espaços no terminal, não importa o tamanho do número".
			// d: Indica que o valor é um número inteiro (decimal). obs: mantei bem alto pra não dar errado
			fmt.Printf(" [VELOCIDADE]: %-6d req/s | [BLOQUEIOS]: %-4d req/s| TOTAL OK: %-8d | TOTAL REJEITADO: %d\n",
				reqPorSegundo, bloqueiosPorSegundo, currentSucesso, currentFalha)
		}
	}()

	fmt.Printf("---- %s Iniciado! ----\n", appName)
	fmt.Println("--- GATEWAY PRONTO NA PORTA :8080 ---")
	http.ListenAndServe(":8080", nil)
}