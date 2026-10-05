# Distribuir o agente em várias máquinas Windows

## 1. Arquivos em cada computador

Use o mesmo executável para computadores com arquitetura compatível. Cada instalação precisa somente destes arquivos:

```text
C:\Monitoramento\
├── monitor-telegram.exe
└── .env
```

Não é necessário instalar Go nas máquinas de destino. O hostname é descoberto automaticamente e aparece nas mensagens para identificar o computador.

Copie o executável gerado em bin/ e um .env com as credenciais e os IDs já configurados. Ajuste TELEGRAM_TOPIC conforme o local:

| Local | Configuração |
|---|---|
| Taiba | TELEGRAM_TOPIC=Taiba |
| Charme | TELEGRAM_TOPIC=charme |
| Magna | TELEGRAM_TOPIC=Magna |
| Acaraizinho | TELEGRAM_TOPIC=acaraizinho |
| Wind | TELEGRAM_TOPIC=wind |

O bot, o grupo e o mapeamento dos tópicos podem ser compartilhados entre as máquinas. Limites e intervalo podem variar por computador. Mantenha o token fora do Git e restrinja a leitura do .env aos responsáveis e à conta que executa o agente.

## 2. Validar antes de automatizar

```powershell
C:\Monitoramento\monitor-telegram.exe -env "C:\Monitoramento\.env" -once -dry-run
```

Depois execute normalmente para validar o destino real quando ocorrer uma transição de alerta:

```powershell
C:\Monitoramento\monitor-telegram.exe -env "C:\Monitoramento\.env"
```

Alertas e recuperações agora incluem uptime. Se a coleta de uptime falhar, a mensagem informa "Uptime: indisponível" sem impedir alertas dos outros recursos.

Encerre a execução manual com Ctrl+C antes de iniciar a tarefa automática, evitando duas instâncias.

## 3. Iniciar automaticamente com o Windows

Para começar sem configurar uma ferramenta de distribuição remota, use uma tarefa no Agendador de Tarefas que inicia o processo no boot. O agente mantém seu próprio loop; não agende uma execução nova a cada 30 segundos.

Execute o seguinte no PowerShell como administrador da máquina de destino, depois de copiar e validar os arquivos. Os comandos registram uma tarefa chamada MonitorTelegram e a iniciam. Se já existir, ela será atualizada; pare a tarefa antes de alterar os arquivos.

```powershell
$ErrorActionPreference = 'Stop'
$agentDirectory = 'C:\Monitoramento'
$agentExecutable = Join-Path $agentDirectory 'monitor-telegram.exe'
$agentEnv = Join-Path $agentDirectory '.env'

if (-not (Test-Path -LiteralPath $agentExecutable -PathType Leaf)) {
    throw 'Executável não encontrado.'
}
if (-not (Test-Path -LiteralPath $agentEnv -PathType Leaf)) {
    throw 'Arquivo .env não encontrado.'
}

$action = New-ScheduledTaskAction -Execute $agentExecutable `
    -Argument ('-env "{0}"' -f $agentEnv) -WorkingDirectory $agentDirectory
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId 'LOCALSERVICE' `
    -LogonType ServiceAccount -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet `
    -ExecutionTimeLimit ([TimeSpan]::Zero) `
    -MultipleInstances IgnoreNew -StartWhenAvailable `
    -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries

Register-ScheduledTask -TaskName 'MonitorTelegram' -Action $action `
    -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName 'MonitorTelegram'
```

A conta LocalService deve ter permissão para ler o executável e o .env e acesso HTTPS à API Telegram. Valide a coleta e a conectividade nessa conta antes da distribuição geral. A tarefa não abre uma janela interativa. A criação do principal usa o [mecanismo documentado pela Microsoft](https://learn.microsoft.com/en-us/powershell/module/scheduledtasks/new-scheduledtaskprincipal).

O limite de execução foi desabilitado para o processo contínuo. IgnoreNew impede duplicação por novos disparos da mesma tarefa; não impede instâncias iniciadas manualmente. Falhas que encerram o processo podem provocar até três reinícios pela tarefa.

Consultar ou parar a tarefa:

```powershell
Get-ScheduledTask -TaskName 'MonitorTelegram'
Get-ScheduledTaskInfo -TaskName 'MonitorTelegram'
Stop-ScheduledTask -TaskName 'MonitorTelegram'
```

O agente atual registra erros no terminal, sem arquivo de log próprio. A simulação e a execução manual ajudam a diagnosticar falhas; a tarefa fornece estado e resultado de execução.

## 4. Onde o Ansible entra

Para instalar pelo Semaphore, consulte o [guia e playbook Ansible](ansible/README.md). Os arquivos de implantação estão agrupados em `docs/ansible/`.

Ansible é útil quando você quer instalar e atualizar muitas máquinas remotamente e já possui acesso de gerenciamento a elas. Ele pode copiar o executável, gerar um .env por host/local e registrar a tarefa acima. O agente continua executando localmente sem depender do Ansible para cada coleta.

Para Windows, prepare um controlador Ansible em Linux e acesso remoto autenticado às máquinas, normalmente com WinRM/PSRP. Use um inventário que associe cada máquina ao seu local e mantenha o token em Ansible Vault ou no gerenciador de segredos utilizado pela equipe. Não coloque tokens no inventário em texto aberto ou nos logs das tarefas de configuração. Requisitos: [gerenciar Windows com Ansible](https://docs.ansible.com/projects/ansible/latest/os_guide/intro_windows.html).

Fluxo de uma instalação automatizada:

1. Confirmar conectividade remota e arquitetura Windows compatível com o binário.
2. Criar C:\Monitoramento com permissões adequadas.
3. Parar a tarefa existente antes de substituir o executável.
4. Copiar o executável e gerar o .env com TELEGRAM_TOPIC do host.
5. Validar uma coleta com -once -dry-run.
6. Criar/atualizar a tarefa de boot e iniciar o agente.
7. Confirmar a operação e o tópico usado por cada computador.

Sem inventário, autenticação e WinRM/PSRP configurados, copiar os dois arquivos e registrar a tarefa por máquina é o primeiro passo mais simples. Nenhum playbook remoto ou alteração de WinRM foi executado neste projeto.

## 5. Atualizações e estado

Atualização de código: pare a tarefa, substitua o .exe e inicie a tarefa novamente. Mudança de limites, intervalo ou local: edite o .env e reinicie a tarefa. Alterações de .env não exigem recompilação.

Cada máquina possui estado de alertas independente em memória. Reiniciar pode emitir um novo alerta se o problema continua ativo. Não use -once na tarefa contínua: ele encerra depois de uma coleta e perde o estado.

O uptime enviado é o tempo desde o boot informado pelo Windows, não o tempo desde que o agente foi iniciado. A tarefa automática e a distribuição remota precisam ser validadas nas máquinas de destino; este guia foi preparado sem instalar tarefas locais ou remotas.
