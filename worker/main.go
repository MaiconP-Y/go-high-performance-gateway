package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

type Contato struct {// estrutura dos contatos
	Nome     string `json:"nome"`
	Telefone string `json:"telefone"`
}

type user struct { // estrutura do usuario puxando a estrutura contato
	Id       int       `json:"id"`
	Nome     string    `json:"nome"`
	Contatos []Contato `json:"contatos"`
}

const hmacSecret = "a4d7014171478639f968070299887ee3c707c17623bbb1e21ba98c3760fe9409"

func main() {
	url := "http://alvo_gateway:8080/post" // albo a ser disparado
	
	// [DADOS]: Criando o usuário com os 4 contatos fictícios
	user := user{
		Id:   1,
		Nome: "Bruto",
		Contatos: []Contato{
			{Nome: "Ana Silva", Telefone: "11999998888"},
			{Nome: "Bruno Souza", Telefone: "21988887777"},
			{Nome: "Carla Dias", Telefone: "31977776666"},
			{Nome: "Diego Lima", Telefone: "41966665555"},
		},
	}
	// O Marshal faz o "Raio-X" da struct e a transforma em JSON.
	// Isso é o conteúdo (Payload) do nosso soco.
	corpoBytes, _ := json.Marshal(user)

	// No Go, o & significa: "Não me dê a coisa, me dê o endereço de onde a coisa está na memória".
	// Você passa apenas o endereço (um número pequeno de 64 bits). É a diferença entre você dar a sua casa para alguém (cópia) ou dar apenas um papel com o seu endereço (ponteiro).
	// O computador só precisa ler o papel e ir até lá. É infinitamente mais rápido.
	client := &http.Client{ // Cria o cliente a partir da biblioteca http em go. 
		Transport: &http.Transport{
			MaxIdleConns:        1000,// Dizemos ao Go: "Mantenha até 1.000 conexões abertas no 'estoque'". Ele controla o estoque de conexões TCP abertas que estão esperando para serem usadas..
			
			// Quantas dessas conexões tcp podem ser usadas para o mesmo servidor de destino.
			//A redução só faz sentido quando o seu http.Client é compartilhado. Se você tem um cliente que fala com "o mundo todo", você precisa de regras de trânsito. Se ele fala com um só, você quer uma pista livre.
			MaxIdleConnsPerHost: 1000,
		},// Sem o conjunto acima: O sistema operacional fica abrindo e fechando portas TCP, gastando CPU e gerando latência (o famoso "atraso" na rede).
	}// MaxIdleConns reserva para usar e o MaxIdleConnsPerHost disponibiliza a reserva

	// O chan (Channel) é um tipo de dado, assim como int, string ou bool. A função dele é apenas uma: transportar coisas de um lado para o outro entre Goroutines de forma segura.
	// Diferente de uma variável comum que só guarda um número, o chan (canal) tem o poder de parar a execução do código.
	// Quando o canal atinge o limite (ex: 1000), ele força a Goroutine a ficar "parada na calçada".
	// Ele precisa saber o que ele vai carregar. struct{}: "Esse mecanismo vai carregar 'nada' (envelopes vazios), serve apenas para sinalização". 1000: "Esse mecanismo tem 1000 vagas (buffer)".
	sem := make(chan struct{}, 1000) // É um sinal pro go de quantas goroutimes usar.
	
	// time.After é o limite de tempo que você aceita ficar esperando na fila antes de desistir.
	// time.Second: É uma constante do Go que vale exatamente 1.000.000.000 (1 bilhão de nanossegundos).
	// *: É o sinal de multiplicação. 30: É o multiplicador.
	timeout := time.After(30 * time.Second)

	for {
		// O select sozinho executa uma única vez e para. 
		// Com um for em volta, dizemos: "Tente fazer isso repetidamente até que algo me faça sair daqui (o return)".
		select {
		// Se o cronômetro de 30 segundos (o timeout) "apitar" ele para no return. Ele é o botão de desligar.
		// Enquanto os 30 segundos estão correndo, o canal está vazio e "fechado para leitura = nil". O select tenta entrar no case <-timeout, vê que não tem nada, e pula direto para o default.
		case <-timeout: 
			return
		default: // No Go, se um select não tem default, ele fica bloqueado esperando um dos cases estar pronto. O default é o que dá a dinâmica de "tentativa constante".
			
			// Essa linha trabalha em conjunto com `sem := make(chan struct{}, 1000)`alem de MaxIdleConns e MaxIdleConnsPerHost
			// Ela é uma trava que permite que não sobrecarregue o worker de envio e o Gateway tambem.
			sem <- struct{}{} 
			go func() { // Função de envio.

				// defer: Ele espera a sua própria Goroutine terminar.
				// O defer é apenas o "segurança" que garante que a porta de saída seja aberta quando o trabalho termina. Quem faz a requisição esperar é a linha sem <- struct{}{} que está antes da go func.
				// defer func() { ... } () O parêntese é o que ativa a função func(). Sem ele, você estaria apenas descrevendo o que fazer, mas não dando a ordem para executar.
				defer func() { <-sem }()

				// Aqui criamos a chave hmac.
				// hmac.New: A ferramenta que cria o motor de cálculo.
				// sha256.New: O algoritmo da segurança.
				// []byte(hmacSecret): A chave secreta separada em bytes.
				h := hmac.New(sha256.New, []byte(hmacSecret))

				// É aqui h.Write(bodyBytes) que a mistura o hmac e do body, literalmente empurramos os dados do JSON para dentro onde ja contem o hmac.
				// O Write é muito foda porque ele não precisa de toda a informação de uma vez só se você não quiser. Você pode dar vários Write seguidos.
				// h.Write: É como colocar os ingredientes no liquidificador e ligar.
				h.Write(corpoBytes)

				// h.Sum(nil): É quando você desliga o liquidificador(h.Write) e despeja o "suco" no copo. Só aqui você tem a assinatura final. Ele é quem faz o trabalho pesado. Ele pega aquela mistura complexa que aconteceu no h.Write e cospe o resultado final do calculo.
				// hex.EncodeToString: Ele olha para esses números e pensa: "Beleza, os humanos (e o Header HTTP) querem ver isso como texto (Hexadecimal)".
				assinatura := hex.EncodeToString(h.Sum(nil))

				// Aqui juntamos as peças, definimos o metodo, a url do alvo. http.NewRequest: Como o proprio nome diz definimos a req.
				// O corpoBytes esta presa na memória ram, ele ta la pronto pra ser usado aqui em bytes.NewBuffer preparamos para mandar pela rede, segue a logica:
				// O corpoBytes é um copo cheio pronto pra ser jogado de uma vez na logica agora a rede só aceita em pedaços não inteiro, ele coloca como se fosse um limite para dividir em pedaços ao ser enviado, como se fosse um bico limitante dentro do codigo.
				req, _ := http.NewRequest("POST", url, bytes.NewBuffer(corpoBytes))

				// Pegamos a req para definir a assisnatura .Set("X-Signature", assinatura), o Header é algo de dentro do *http.Request que define o header ja pronto, é basicamente um Mapa (map[string][]string) que já está preenchido na memória RAM antes mesmo do código começar a rodar. Pemitindo mais de uma assinatura a partir de []string.
				// O Go já tem o modelo do envelope definido, os selos em uma pastinha organizada. ele permite definir sem esforço.
				// Com o .Set aplicamos nosso resultado de calculo do corpo bytes e hmac na req.
				req.Header.Set("X-Signature", assinatura)

				resp, err := client.Do(req) // Aqui damos o soco, disparamos a req a partir do client http do go, o do é a função em especifica pronta pro soco.
				
				// Se err voltar vazio não ouve erro e finaliza o envio em resp.Body.Close() ele fecha a porta tcp para não ficar aberta e gastando recurso.
				// Go entende em resp.Body.Close(): "Opa, o Maicon terminou de usar essa conexão TCP. Vou limpar ela e devolver para o estoque (Idle Pool) em vez de jogar fora".
				if err == nil {
					// Body: É oque pega a resposta pega o header, mas ignora o body que é o json de resposta.
					// O Header chega primeiro: O Go lê os metadados (Status 200, Content-Type, etc.) e coloca na struct resp. Isso é leve e rápido.
					resp.Body.Close()
					// Você só pode fechar o Body se a resposta existir. Se der um erro (o servidor alvo estiver desligado, por exemplo), a variável resp estará vazia (nil), e tentar fechar algo que não existe faria o programa "crashar".
					// Se o servidor estiver desligado ou der erro ele vai crashar, se ocorrer isso ele dispara e ja era. O erro acontece e deve ter uma logica no caso de produção para não bugar.
				}
			}()
		}
	}
}