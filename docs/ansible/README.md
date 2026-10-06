# Instalar pelo Ansible no Semaphore

O Semaphore executa o playbook; o agente roda continuamente em cada Windows. Não agende o playbook a cada 30 segundos. Ele serve para instalação e atualização.

O fluxo é um hotel por execução: conecte a VPN do hotel, execute seu template, aguarde a conclusão e só então troque a VPN para o próximo hotel. Depois de instalado, o agente continua rodando sem a VPN de implantação, desde que o próprio computador tenha acesso HTTPS ao Telegram.

A VPN precisa dar acesso aos computadores a partir do **servidor ou runner que executa o Ansible**. Conectar a VPN somente no seu notebook não dá acesso ao Semaphore instalado em outro servidor, a menos que esse servidor tenha uma rota configurada pela VPN. Se o runner estiver em container, valide também suas rotas para a rede do hotel.

## 1. Preparar a conexão

Use um runner Linux com Ansible e Python. Instale `pywinrm` no mesmo ambiente Python utilizado pelo Ansible; se usar Docker, inclua a dependência na imagem do runner para não perdê-la ao recriar o container:

```bash
python3 -m pip install 'pywinrm>=0.4.0'
ansible-galaxy collection install -r docs/ansible/collections/requirements.yml
```

O playbook usa `ansible.windows` e `community.windows`. O Semaphore procura `collections/requirements.yml` ao lado do playbook e pode instalar essas coleções automaticamente.

Cada destino precisa ser Windows x64, ter PowerShell 5.1 ou superior e aceitar a conexão WinRM já utilizada no seu Semaphore. O playbook utiliza as configurações de conexão, usuário e senha dos seus inventários existentes. A conta de automação precisa de privilégios administrativos no destino. Este playbook não altera os inventários, não habilita WinRM nem altera firewall ou políticas de autenticação.

As máquinas também precisam de acesso HTTPS a `api.telegram.org`. Ambientes que bloqueiam NTLM exigem outra autenticação, como Kerberos, preparada no runner.

Referência: [WinRM com Ansible](https://docs.ansible.com/projects/ansible/latest/os_guide/windows_winrm.html).

## 2. Disponibilizar os arquivos no repositório

O Semaphore precisa de um repositório de implantação com:

```text
bin/monitor-telegram.exe
docs/ansible/install.yml
docs/ansible/collections/requirements.yml
```

Compile neste projeto no Windows x64:

```powershell
go build -o bin/monitor-telegram.exe .
```

O diretório `bin/` está ignorado pelo Git. Para esta implantação simples, inclua o executável explicitamente no repositório de implantação, junto dos arquivos Ansible:

```bash
git add docs/ansible
git add -f bin/monitor-telegram.exe
```

Depois faça commit e publique na branch escolhida no Semaphore. Esses comandos pressupõem um repositório Git já inicializado; eles não foram executados aqui. Não inclua o `.env` local. O executável não contém o token: o playbook gera o `.env` em cada destino usando o segredo do Semaphore. Atualizações frequentes de binários aumentam o histórico Git; nesse caso, disponibilize o artefato em um volume do runner e informe seu caminho absoluto em `agent_binary_src` nas Extra Variables.

## 3. Configurar o Semaphore

1. **Repositories:** cadastre o repositório e a branch. Se for privado, associe sua credencial em Key Store.
2. **Inventory:** selecione o inventário do hotel que já existe no Semaphore. Não edite hosts, logins, senhas nem variáveis do inventário. O playbook atende ao grupo `windows_hosts`, presente no inventário fornecido. Use somente seus inventários existentes no Semaphore.
3. **Variable Groups / Environment:** reutilize o grupo de variáveis que já fornece as credenciais daquele hotel. Preserve os segredos atuais e acrescente `TELEGRAM_BOT_TOKEN` na aba **Secrets**, como variável de ambiente para o processo Ansible. Não é necessário criar `WIN_PASSWORD`. Não coloque senhas ou token nas Extra Variables nem no Git.
4. **Extra Variables:** configure o hotel do template por `agent_topic`. Para Taiba:

```json
{
  "agent_topic": "Taiba",
  "agent_cpu_limit": 90,
  "agent_ram_limit": 70,
  "agent_disk_limit": 90,
  "agent_interval": "30s"
}
```

5. **Task Templates:** crie um template **Ansible Playbook** para cada hotel, como `Instalar agente - Taiba` e `Instalar agente - Charme`, associando cada um ao seu inventário existente e ao grupo de variáveis correspondente. Todos usam o mesmo repositório e playbook: `docs/ansible/install.yml`. Configure as Extra Variables de cada template/grupo conforme a tabela abaixo, preservando as demais variáveis já utilizadas. Preserve também a configuração de credenciais de acesso que já funciona no seu Semaphore.
6. Conecte a VPN Taiba no ambiente do runner e execute `Instalar agente - Taiba`. Na primeira validação, use **Limit** = `TAIBA-PDV-PAP0`; depois remova o Limit para instalar no inventário inteiro. Aguarde a tarefa terminar, conecte a VPN Charme e execute `Instalar agente - Charme`. Repita para os demais hotéis. Não execute templates de hotéis diferentes ao mesmo tempo enquanto troca a VPN. Não use Check Mode neste fluxo: a coleta precisa dos arquivos realmente instalados.

| Template | Inventário existente | Extra Variables |
|---|---|---|
| Instalar agente - Taiba | Taiba | `{"agent_topic":"Taiba"}` |
| Instalar agente - Charme | Charme | `{"agent_topic":"Charme"}` |
| Instalar agente - Magna | Magna | `{"agent_topic":"Magna"}` |
| Instalar agente - Acaraizinho | Acaraizinho | `{"agent_topic":"Acaraizinho"}` |
| Instalar agente - Wind | Wind | `{"agent_topic":"Wind"}` |

Não precisa criar subgrupos ou usar Limit para selecionar o hotel: o inventário existente já delimita os computadores. Todos os hosts da execução recebem o tópico das Extra Variables. Se sua versão do Semaphore armazena Extra Variables no grupo de variáveis, use um grupo por hotel, preservando as credenciais necessárias em cada um; não altere um grupo compartilhado para mudar o tópico de vários templates. A seleção do hotel é explícita pelo template; o playbook não tenta identificar o hotel pela VPN, pelo IP ou pelo hostname. Sem um tópico válido, a instalação falha antes de alterar os computadores.

Documentação: [Inventários](https://semaphoreui.com/docs/user-guide/inventory), [grupos de variáveis e segredos](https://semaphoreui.com/docs/user-guide/environment), [repositórios e coleções](https://semaphoreui.com/docs/user-guide/repositories).

## 4. Comportamento da instalação

Para cada host, um de cada vez, o playbook:

Antes das demais tarefas, testa WinRM. Se o resultado for `UNREACHABLE`, informa o host pulado, encerra somente sua execução e continua no próximo computador. Isso cobre máquinas offline e outras falhas de conexão, inclusive erros de autenticação classificados como `UNREACHABLE`: consulte a causa no log. O computador pulado não recebe instalação ou atualização nesta execução; execute novamente quando estiver acessível.

1. Valida os segredos, o tópico e a presença do executável no runner.
2. Confirma conexão e arquitetura x64.
3. Cria `C:\Monitoramento`, permitindo acesso somente a Administradores, SYSTEM e LocalService. LocalService recebe leitura e execução.
4. Para a tarefa `MonitorTelegram`, se estiver rodando.
5. Copia o executável e gera `.env` com o tópico do host e os limites definidos.
6. Executa uma coleta `-once -dry-run` sem mensagens reais.
7. Cria ou atualiza a tarefa de boot como LocalService, sem limite de duração e sem duplicar instâncias da mesma tarefa.
8. Inicia a tarefa e verifica se continua em execução após cinco segundos.

O grupo padrão é `-1004428450910`; os IDs são Taiba 26, Charme 27, Magna 29, Acaraizinho 30 e Wind 28. Esses IDs pertencem ao grupo atual. Outro grupo exige alterar `telegram_chat_id` e `telegram_topics`.

O modo `dry-run` valida limites e coleta, mas não verifica autenticação do Telegram nem entrega ao tópico. A checagem final confirma o processo, não a entrega de mensagens. Valide um alerta real no primeiro computador antes de distribuir para todos. O playbook não envia uma mensagem de teste.

Encerre instâncias iniciadas manualmente antes da implantação. O playbook gerencia somente a tarefa `MonitorTelegram`. O diretório deve ser dedicado ao agente; arquivos antigos com permissões explícitas precisam de revisão, pois a ACL da pasta controla a herança dos novos arquivos.

## 5. Atualizar e diagnosticar

Para mudar código, recompile, publique o novo executável no repositório de implantação e execute o mesmo template. Para mudar limites, altere as variáveis e execute novamente. Limites individuais podem ser definidos diretamente em cada host do inventário, por exemplo `agent_ram_limit: 80`, desde que não sejam sobrescritos nas Extra Variables (que têm maior precedência).

Cada execução reinicia o agente, mesmo sem alteração de arquivos. Isso reinicia o estado dos alertas e pode emitir um novo alerta para um problema ainda ativo. Máquinas inacessíveis no teste inicial são puladas; outros erros continuam interrompendo a distribuição. Se falhar depois de parar a tarefa, o computador pode ficar sem monitoramento até corrigir e executar novamente; não há rollback automático.

No Windows, consulte:

```powershell
Get-ScheduledTask -TaskName MonitorTelegram
Get-ScheduledTaskInfo -TaskName MonitorTelegram
C:\Monitoramento\monitor-telegram.exe -env C:\Monitoramento\.env -once -dry-run
```

Os arquivos foram preparados localmente. A execução do playbook, a autenticação WinRM e a instalação via Semaphore precisam ser validadas no runner e em uma máquina de destino.

## 6. Desinstalar pelo Semaphore

Publique também `docs/ansible/uninstall.yml` no mesmo repositório. Crie um template **Ansible Playbook** por hotel, como `Desinstalar agente - Taiba`, usando o inventário existente e o grupo de variáveis que já fornece as credenciais Windows. No campo do playbook, informe:

```text
docs/ansible/uninstall.yml
```

Não é necessário token do Telegram, `agent_topic` ou executável no runner para desinstalar. Conecte a VPN do hotel no ambiente do runner e execute primeiro em uma máquina com **Limit**; depois remova o Limit para o inventário inteiro. Aguarde a conclusão antes de trocar de VPN. A conta de acesso precisa de privilégios administrativos.

A desinstalação também pula hosts com `UNREACHABLE` no teste inicial e continua nos demais. Nesses computadores, a remoção permanece pendente e o agente instalado continua presente. Execute novamente quando estiverem acessíveis.

O playbook desabilita e para `MonitorTelegram`, remove essa tarefa, encerra instâncias manuais cujo executável esteja em `C:\Monitoramento\monitor-telegram.exe` e apaga toda a pasta `C:\Monitoramento`, incluindo `.env`, executável e demais arquivos dentro dela. A pasta deve ser exclusiva do agente. Ele verifica a ausência da tarefa, dos processos e da pasta ao terminar. Executar novamente em uma máquina já desinstalada não altera nada.

Por segurança, a exclusão é limitada ao caminho padrão `C:\Monitoramento`; caminhos diferentes, links/junctions e tarefas com ações inesperadas bloqueiam a operação. Instalações personalizadas em outro caminho exigem adaptar e revisar essa validação antes de executar.

As mensagens já enviadas ao Telegram, o histórico do Semaphore e os registros de auditoria do Windows permanecem. Eles não são componentes da instalação e não são apagados pelo playbook. A desinstalação remota ainda precisa ser validada no seu ambiente.
