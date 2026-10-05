# Enviar mensagens para o Telegram com Python

Este guia serve para qualquer automação: confirmar um pedido, avisar que uma tarefa terminou, comunicar uma aprovação ou informar um erro.

A ideia é simples:

```text
Sua automação → mensagem → bot do Telegram → grupo ou tópico
```

## 1. O que você precisa

- Um bot do Telegram e seu token.
- O ID do grupo que receberá as mensagens.
- O ID do tópico, caso queira enviar para um tópico específico.

O **token** identifica o bot. O **ID do grupo** escolhe a conversa. O **ID do tópico** escolhe uma área dentro dessa conversa.

Crie um bot em [@BotFather](https://t.me/BotFather), usando /newbot, ou obtenha o token do bot existente com seu responsável. Adicione o bot ao grupo e permita que ele envie mensagens.

## 2. Organize os arquivos

Copie o [exemplo Python](telegram_topics.py) para a pasta da sua automação:

```text
minha-automacao/
├── minha_automacao.py
├── telegram_topics.py
└── .env
```

O módulo não depende do agente Go nem de funções de monitoramento.

## 3. Configure o .env

```dotenv
TELEGRAM_BOT_TOKEN=8789549979:AAErzdEsDlW4tjNDZ9Vn7uSNPiNkJ0dFFWI
TELEGRAM_CHAT_ID=-1004428450910
TELEGRAM_CHAT_ID=ID_DO_GRUPO
TELEGRAM_TOPIC_ID=ID_DO_TOPICO
```

Para enviar ao grupo sem escolher um tópico, deixe TELEGRAM_TOPIC_ID vazio.

Não coloque o token no código ou no Git. Acrescente .env ao .gitignore. Após alterar o arquivo, reinicie sua automação.

### Exemplo com o grupo/Tópico

Grupo **Monitoramento**: -1004428450910.

| Tópico | ID |
|---|---:|
| Taiba | 26 |
| Charme | 27 |
| Wind | 28 |
| Magna | 29 |
| Acaraizinho | 30 |

Para enviar à Magna:

```dotenv
TELEGRAM_BOT_TOKEN=SEU_TOKEN_AQUI
TELEGRAM_CHAT_ID=-1004428450910
TELEGRAM_TOPIC_ID=29
```

Esses IDs valem somente para esse grupo e esses tópicos. Em outro grupo, use os IDs dele. Para descobrir os IDs, envie uma mensagem ao bot dentro do tópico e consulte [getUpdates](https://core.telegram.org/bots/api#getupdates): chat.id é o grupo e message_thread_id é o tópico. Uma mensagem privada ao bot não identifica o grupo.

O exemplo Python usa TELEGRAM_TOPIC_ID diretamente. Não é necessário copiar o mapeamento por nome do agente Go.

## 4. Instale o necessário

No terminal do projeto Python:

```powershell
python -m pip install python-dotenv
```

Essa biblioteca lê o .env. O envio HTTP usa a biblioteca padrão do Python.

Se a automação já tem um ambiente virtual, execute o comando usando o Python desse ambiente.

## 5. Teste a configuração

```powershell
python telegram_topics.py --env ".env" --dry-run
```

Esse comando mostra o grupo e o tópico escolhidos, sem enviar mensagens. Ele não verifica o token nem as permissões do bot.

Para enviar uma mensagem de teste:

```powershell
python telegram_topics.py --env ".env" --text "Olá! Teste da minha automação."
```

Confira se a mensagem chegou ao destino correto.

## 6. Use na sua automação

Exemplo completo de minha_automacao.py:

```python
from pathlib import Path

from dotenv import load_dotenv
from telegram_topics import TelegramError, send_to_topic

# Carrega o .env que está na mesma pasta deste script.
load_dotenv(Path(__file__).resolve().with_name(".env"), override=False)

try:
    send_to_topic("✅ O pedido 123 foi aprovado.")
except ValueError as error:
    print(f"Confira a configuração: {error}")
except TelegramError as error:
    print(f"Não foi possível enviar: {error}")
    if error.retry_after:
        print(f"Aguarde {error.retry_after} segundos antes de tentar novamente.")
else:
    print("Mensagem enviada com sucesso.")
```

Troque o texto pela mensagem que sua automação precisa enviar. Chame send_to_topic no momento desejado: depois de concluir uma tarefa, ao aprovar algo ou quando ocorrer um erro.

Importar o módulo não envia mensagens. Cada chamada envia uma mensagem; decidir quando chamar e evitar repetições fica a cargo da sua automação.

Variáveis já definidas no ambiente têm prioridade sobre o .env, porque o carregamento usa override=False.

## 7. O que acontece por trás

O módulo faz uma requisição [sendMessage](https://core.telegram.org/bots/api#sendmessage) com estes dados:

```text
POST https://api.telegram.org/bot<TOKEN>/sendMessage
Content-Type: application/x-www-form-urlencoded

chat_id: ID do grupo
message_thread_id: ID do tópico, quando configurado
text: sua mensagem
```

O token fica na URL da API. O módulo codifica o texto e verifica a resposta antes de confirmar sucesso. Ele usa timeout de 10 segundos e trata erros sem mostrar o token.

Se houver limite de envios, TelegramError.retry_after informa quanto aguardar. O exemplo não tenta novamente automaticamente.

## 8. Se não funcionar

| Situação | O que conferir |
|---|---|
| Token inválido | Credencial fornecida pelo BotFather |
| Mensagem não chega | ID do grupo e participação/permissões do bot |
| Mensagem vai ao lugar errado | TELEGRAM_TOPIC_ID e grupo correspondente |
| Configuração não mudou | Reinício e variáveis antigas no ambiente |
| Erro ao ler .env | Caminho do arquivo, extensão .env.txt e instalação de python-dotenv |

O exemplo foi verificado com respostas HTTP simuladas, sem enviar mensagens reais. O carregamento de .env com python-dotenv não foi executado neste ambiente: o download da dependência falhou. O responsável deve confirmar a instalação da biblioteca e a entrega de uma mensagem de teste.
