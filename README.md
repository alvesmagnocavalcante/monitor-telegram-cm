# Monitor Telegram em Go

Agente local para Windows: CPU, RAM, ocupação do disco C: e uptime. Sem banco de dados, API ou interface web.

O [guia de envio pelo Telegram em Python](docs/GUIA_REPLICACAO.md) explica como enviar mensagens de qualquer automação, com um [exemplo reutilizável](docs/telegram_topics.py).

Para instalar o agente em várias máquinas, consulte [distribuição Windows](docs/DISTRIBUICAO_WINDOWS.md): configuração por local, tarefa de inicialização e uso de Ansible.

## Executar no PowerShell

1. Instale o Go (este projeto usa Go 1.27.1 ou superior).
2. Crie um bot conversando com [@BotFather](https://t.me/BotFather), usando /newbot. Guarde o token.
3. Para chat privado, abra a conversa com o bot e envie /start. Para grupo, adicione o bot e envie um comando como /start@NomeDoSeuBot. Garanta permissão para enviar mensagens.
4. Abra o arquivo .env na raiz do projeto e preencha suas credenciais (use .env.example como modelo):

```dotenv
TELEGRAM_BOT_TOKEN=TOKEN_DO_SEU_BOT
TELEGRAM_CHAT_ID=ID_DO_CHAT_OU_GRUPO
```

Depois execute:

```powershell
Set-Location 'C:\Users\magno.alves\Documents\projetos\telegram-report'
go mod download
go run .
```

Para descobrir o chat ID, após enviar a mensagem ao bot:

```powershell
$botToken = Read-Host "Token do bot para consultar o chat ID"
$updates = Invoke-RestMethod -Uri ("https://api.telegram.org/bot{0}/getUpdates" -f $botToken)
$updates.result | ForEach-Object { $_.message.chat } | Select-Object id, title, username, type
```

Copie o campo id, incluindo o sinal negativo nos grupos. Se não houver resultados, envie outra mensagem ao bot e repita. getUpdates não funciona enquanto o bot possui um webhook ativo. Use um bot dedicado a este projeto.

O programa procura primeiro o .env ao lado do executável; se não existir, usa o .env do diretório atual. Com go run ., normalmente usa o arquivo da raiz do projeto. Use -env para escolher outro arquivo explicitamente. Basta preencher o arquivo uma vez; reinicie o agente depois de mudar as credenciais. O .env está no .gitignore; somente .env.example deve ser versionado. Não publique credenciais.

Variáveis já definidas no PowerShell têm prioridade sobre o .env. Se quiser usar somente o arquivo na sessão atual, remova os valores antigos:

```powershell
Remove-Item Env:TELEGRAM_BOT_TOKEN, Env:TELEGRAM_CHAT_ID -ErrorAction SilentlyContinue
```

O .env é opcional quando as duas variáveis já estão configuradas no ambiente. Sem ambas as credenciais, o programa termina com erro. Um arquivo .env inválido também encerra a inicialização, sem expor as linhas com segredos.

Encerre com **Ctrl+C**.

## Distribuir computadores por tópicos

Use um grupo do Telegram com tópicos habilitados e crie os tópicos **Taiba**, **charme**, **Magna**, **acaraizinho** e **wind**. Adicione o bot e permita que ele envie mensagens nesses tópicos.

Cada computador envia seus alertas e recuperações para **um único tópico**. O ID do grupo é o mesmo em todos; o nome selecionado varia conforme o local do computador. O Telegram recebe o ID do tópico pelo parâmetro [message_thread_id](https://core.telegram.org/bots/api#sendmessage).

Exemplo de configuração de um computador da Taiba (substitua os valores de exemplo pelos seus IDs reais):

```dotenv
TELEGRAM_BOT_TOKEN=TOKEN_DO_BOT
TELEGRAM_CHAT_ID=ID_DO_GRUPO
TELEGRAM_TOPIC=Taiba
TELEGRAM_TOPIC_TAIBA_ID=ID_NUMERICO_DO_TOPICO_TAIBA
TELEGRAM_TOPIC_CHARME_ID=
TELEGRAM_TOPIC_MAGNA_ID=
TELEGRAM_TOPIC_ACARAIZINHO_ID=
TELEGRAM_TOPIC_WIND_ID=
```

Os nomes aceitos são Taiba, charme, Magna, acaraizinho e wind, sem distinção entre maiúsculas e minúsculas. Preencha ao menos o ID do tópico selecionado. Você pode preencher os cinco IDs em um modelo comum e mudar somente TELEGRAM_TOPIC ao instalar em cada computador.

TELEGRAM_CHAT_ID identifica o grupo; TELEGRAM_TOPIC_*_ID identifica um tópico dentro dele. O nome do tópico sozinho não permite ao bot localizar seu ID. Se o nome for desconhecido ou o ID selecionado estiver ausente/inválido, o agente encerra com erro, evitando enviar ao tópico Geral por engano. Deixe TELEGRAM_TOPIC vazio apenas quando quiser o envio direto ao chat, como na configuração anterior.

### Obter os IDs

Dentro de cada tópico, envie um comando ao bot incluindo o local, por exemplo:

```text
/start@NomeDoSeuBot Taiba
```

Repita nos outros tópicos com seus respectivos nomes. Depois consulte as atualizações no PowerShell:

```powershell
$botToken = Read-Host "Token do bot para consultar os tópicos"
$updates = Invoke-RestMethod -Uri ("https://api.telegram.org/bot{0}/getUpdates" -f $botToken)
$updates.result | ForEach-Object {
    if ($_.message) {
        [pscustomobject]@{
            ChatID = $_.message.chat.id
            TopicID = $_.message.message_thread_id
            Texto = $_.message.text
        }
    }
} | Format-Table -AutoSize
```

Associe cada Texto ao seu TopicID e copie esses IDs para o .env. Se TopicID estiver vazio, confira se a mensagem foi enviada dentro do tópico. getUpdates requer um bot sem webhook ativo; use um bot dedicado para evitar que outro serviço consuma as atualizações.

### Distribuir o executável

Compile uma vez:

```powershell
go build -o bin/monitor-telegram.exe .
```

Em cada computador, copie bin/monitor-telegram.exe e o .env correspondente para a mesma pasta. Abra o PowerShell nessa pasta e execute:

```powershell
.\monitor-telegram.exe
```

Na distribuição, o .env ao lado do executável é carregado mesmo quando o agente é iniciado de outra pasta. Alterar TELEGRAM_TOPIC ou os IDs exige apenas reiniciar o agente; não precisa recompilar. Variáveis já existentes no ambiente têm prioridade sobre o arquivo.

## Opções de execução

Inspiradas no [agente Humand de referência](https://github.com/alvesmagnocavalcante/monitor-humand-agent/blob/main/agent.go), adaptadas ao envio Telegram e suas mensagens de recuperação.

- -env caminho: seleciona um arquivo .env. Um caminho explícito inexistente gera erro.
- -once: coleta uma vez, processa os alertas e encerra. Falhas de envio retornam código de saída de erro.
- -dry-run: imprime as métricas e os alertas/recuperações simulados, sem usar a API Telegram. Não exige token, chat ou tópico.

Teste local sem envio:

```powershell
.\bin\monitor-telegram.exe -once -dry-run
```

Escolha outro arquivo:

```powershell
.\bin\monitor-telegram.exe -env "C:\Monitoramento\.env"
```

Para desenvolvimento:

```powershell
go run . -once -dry-run
```

O monitor continua usando DISK_PATH. Leituras NaN, infinitas ou fora de 0 a 100 não geram alertas nem falsas recuperações. O estado só muda após um envio confirmado, mantendo novas tentativas em caso de falha. No modo simulação, o estado acompanha apenas as mensagens exibidas no terminal.

## Saída

```text
Monitor iniciado...

Host: MEU-PC
CPU: 34.2%
RAM: 61.5%
Disco C: 72.1%
Uptime: 3d 4h 21m

Próxima verificação em 30 segundos...
```

A primeira leitura começa imediatamente. CPU é medida durante uma janela de um segundo e representa o uso agregado dos processadores. Disco representa espaço ocupado, não atividade de leitura/escrita. Uptime representa o tempo desde o boot informado pelo Windows.

Após a coleta e os envios, o programa espera o intervalo configurado em MONITOR_INTERVAL (30 segundos por padrão). Portanto, o tempo entre leituras inclui a coleta e as requisições HTTP.

## Limites e alertas

Configure no .env:

```dotenv
CPU_LIMIT=90
RAM_LIMIT=75
DISK_LIMIT=90
MONITOR_INTERVAL=30s
DISK_PATH=C:\
```

- Limites aceitam valores de 0 a 100; use ponto para decimais, como 75.5.
- MONITOR_INTERVAL aceita durações positivas, como 30s, 1m ou 2m30s.
- DISK_PATH aceita um caminho absoluto. No .env, escreva C:\ sem duplicar a barra; C:/ também funciona no Windows.
- Variáveis ausentes ou vazias usam os valores padrão mostrados acima.
- Valores inválidos encerram a inicialização com uma mensagem indicando a variável.
- Variáveis já definidas no ambiente têm prioridade sobre o .env.

O alerta ocorre quando o valor é **maior** que o limite configurado. No limite ou abaixo dele, o recurso está normal.

Altere o .env e reinicie o agente para aplicar novos valores. Não precisa recompilar após mudanças de configuração; esta atualização do código exige substituir o executável antigo pelo novo em bin/.

Cada recurso possui estado independente em um map em memória:

- Normal → acima do limite: envia um alerta.
- Continua acima do limite: não repete o alerta.
- Alerta → normal: envia recuperação.
- Ultrapassa o limite novamente: envia um novo alerta.

O estado é confirmado somente após resposta de sucesso do Telegram. Se o envio falhar, a próxima coleta tenta novamente caso a transição ainda seja necessária. Uma falha de leitura é exibida no terminal e não altera o estado; as outras métricas continuam sendo processadas.

Alertas e recuperações incluem o uptime da máquina. Quando sua coleta falha, a mensagem mostra "Uptime: indisponível".

O cliente HTTP tem timeout de 10 segundos, respeita retry_after, limita o tamanho da resposta e remove o token dos erros. O estado não é persistido: reiniciar o agente pode gerar novos alertas. Em caso de resposta perdida após a entrega, uma nova tentativa pode duplicar a mensagem; a API sendMessage não fornece garantia de entrega única.

## Organização e conceitos de Go

A raiz contém a configuração e o ponto de entrada. A implementação e seus testes ficam em internal/agent/, a documentação em docs/ e o executável em bin/.

- main.go: ponto de entrada do executável.
- internal/agent/agent.go: hostname, inicialização, configuração de tópicos e loop.
- internal/agent/config.go: leitura e validação da configuração de monitoramento.
- internal/agent/monitor.go: coleta, impressão, comparação de limites e estado dos alertas.
- internal/agent/telegram.go: HTTP POST para a Telegram Bot API.
- internal/agent/*_test.go: testes locais, sem bot ou credenciais reais.
- go.mod / go.sum: versões das dependências e verificações de integridade.

package main e func main() definem o executável e seu ponto de entrada. := declara variáveis com tipos inferidos; funções podem retornar um valor e um error. err != nil verifica falhas.

struct agrupa dados de cada leitura. map[string]bool guarda se um alerta já foi enviado para cada recurso. O for sem condição executa continuamente.

time.Duration representa um intervalo; 30 * time.Second é uma duração de 30 segundos. time.Sleep bloquearia a espera até seu término. Aqui, time.NewTimer e select permitem esperar pelo intervalo ou pelo cancelamento, encerrando prontamente com Ctrl+C.

signal.NotifyContext cancela o context.Context ao receber Ctrl+C. Esse contexto é repassado à coleta e ao HTTP para interromper operações em andamento.

O envio segue: **programa Go → HTTP POST → Telegram Bot API → chat/grupo**. O formulário contém chat_id e text; o programa verifica tanto o status HTTP quanto o campo ok da resposta.

As dependências diretas são [gopsutil/v4](https://github.com/shirou/gopsutil), para coleta do sistema, e [godotenv](https://github.com/joho/godotenv), para ler .env sem implementar um parser próprio. Suas dependências transitivas ficam registradas pelo Go. O cliente Telegram usa somente a biblioteca padrão e segue a [documentação de sendMessage](https://core.telegram.org/bots/api#sendmessage).

## Validar e compilar

```powershell
go test ./...
go vet ./...
go build -o bin/monitor-telegram.exe .
.\bin\monitor-telegram.exe
```

Os testes cobrem limites, alertas independentes, ausência de spam, recuperação, novas tentativas, erros de coleta, cancelamento, redirecionamentos, respostas HTTP inválidas e limitação do Telegram. No Windows, também coletam as métricas reais.

Para testar uma notificação real, reduza temporariamente RAM_LIMIT no .env para um valor abaixo do uso atual, reinicie o agente e aguarde o alerta. Depois, configure acima do uso atual e reinicie para testar a configuração; reiniciar perde o estado, portanto não testa recuperação. Para observar recuperação na mesma execução, mantenha o limite fixo e faça o consumo do recurso passar de acima para abaixo dele.

Os testes HTTP usam um servidor local e não enviam mensagens reais.
