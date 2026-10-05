"""Cliente Telegram para integrar à automação Python. Não envia ao importar."""

import argparse
import json
import os
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import HTTPRedirectHandler, Request, build_opener

MAX_RESPONSE_BYTES = 64 * 1024


class TelegramError(RuntimeError):
    def __init__(self, message, retry_after=0):
        super().__init__(message)
        self.retry_after = retry_after


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _destination(require_token=True):
    token = os.getenv("TELEGRAM_BOT_TOKEN", "").strip()
    chat_id = os.getenv("TELEGRAM_CHAT_ID", "").strip()
    if require_token and (not token or any(c in token for c in "/?# \t\r\n")):
        raise ValueError("Configure TELEGRAM_BOT_TOKEN com um token válido.")
    if not chat_id:
        raise ValueError("Configure TELEGRAM_CHAT_ID.")
    topic_id = os.getenv("TELEGRAM_TOPIC_ID", "").strip()
    thread_id = None
    if topic_id:
        try:
            thread_id = int(topic_id)
        except ValueError:
            raise ValueError("TELEGRAM_TOPIC_ID deve ser um inteiro positivo.") from None
        if not 0 < thread_id <= 2**63 - 1:
            raise ValueError("TELEGRAM_TOPIC_ID deve ser um inteiro positivo.")
    return token, chat_id, thread_id


def send_to_topic(text):
    """Envia uma mensagem e retorna result. Só confirma sucesso se ok for True."""
    if not isinstance(text, str) or not 1 <= len(text) <= 4096:
        raise ValueError("A mensagem deve conter de 1 a 4096 caracteres.")
    token, chat_id, thread_id = _destination()
    fields = {"chat_id": chat_id, "text": text}
    if thread_id is not None:
        fields["message_thread_id"] = thread_id
    request = Request(
        f"https://api.telegram.org/bot{token}/sendMessage",
        data=urlencode(fields).encode("utf-8"),
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )
    opener = build_opener(_NoRedirect())
    try:
        response = opener.open(request, timeout=10)
    except HTTPError as error:
        response = error  # Permite ler a resposta JSON, inclusive retry_after.
    except (URLError, OSError):
        raise TelegramError("Falha de conexão; envio não confirmado.") from None

    try:
        with response:
            status = response.getcode()
            raw = response.read(MAX_RESPONSE_BYTES + 1)
    except OSError:
        raise TelegramError("Falha ao ler resposta; envio não confirmado.") from None
    if len(raw) > MAX_RESPONSE_BYTES:
        raise TelegramError("Resposta do Telegram excedeu o limite de tamanho.")
    try:
        payload = json.loads(raw)
    except (ValueError, UnicodeError):
        raise TelegramError(f"Resposta inválida do Telegram (HTTP {status}).") from None
    if not isinstance(payload, dict):
        raise TelegramError("Resposta do Telegram não é um objeto JSON.")
    if not 200 <= status < 300 or payload.get("ok") is not True:
        description = str(payload.get("description", "Envio recusado.")).replace(token, "[TOKEN REMOVIDO]")
        parameters = payload.get("parameters")
        retry_after = parameters.get("retry_after", 0) if isinstance(parameters, dict) else 0
        if type(retry_after) is not int or retry_after < 0:
            retry_after = 0
        raise TelegramError(f"Telegram HTTP {status}: {description}", retry_after)
    result = payload.get("result")
    if not isinstance(result, dict):
        raise TelegramError("Resposta de sucesso sem result válido.")
    return result


def main():
    parser = argparse.ArgumentParser(description="Teste de envio para o tópico configurado.")
    parser.add_argument("--env", type=Path, help="Caminho do arquivo .env.")
    parser.add_argument("--dry-run", action="store_true", help="Valida o destino sem enviar.")
    parser.add_argument("--text", default="Teste de integração da automação Python")
    args = parser.parse_args()
    env_path = args.env
    if env_path is None:
        adjacent = Path(__file__).resolve().with_name(".env")
        env_path = adjacent if adjacent.is_file() else Path.cwd() / ".env"
    if args.env is not None and not env_path.is_file():
        parser.error("O arquivo informado em --env não existe ou não é um arquivo.")
    if env_path.is_file():
        try:
            from dotenv import load_dotenv
        except ImportError:
            parser.error("Para ler .env, instale: python -m pip install python-dotenv")
        load_dotenv(env_path, override=False)
    try:
        _, chat_id, thread_id = _destination(require_token=not args.dry_run)
        if args.dry_run:
            print(f"Simulação: chat {chat_id}, tópico {thread_id or 'não selecionado'}; nenhuma mensagem enviada.")
            return 0
        result = send_to_topic(args.text)
        print(f"Mensagem confirmada; ID: {result.get('message_id', 'não informado')}.")
        return 0
    except (ValueError, TelegramError) as error:
        print(str(error), file=sys.stderr)
        if isinstance(error, TelegramError) and error.retry_after:
            print(f"Aguarde ao menos {error.retry_after} segundos antes de tentar novamente.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
