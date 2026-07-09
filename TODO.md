# TODO

## Revisão de pronúncia — STT (implementado nesta sessão; falta publicar)

- [ ] **Publicar o motor de escuta Linux**: empurrar a tag `motores-stt-linux-v1` (workflow
      `publicar-motores-stt-linux.yml` congela o `paraformer_server_linux.zip`, cria a release e
      atualiza `wails_app/motoresstt/artefatos_stt_linux.json` sozinho). Até lá a UI mostra o motor
      como "Indisponível" (sha256 vazio no manifesto).
- [ ] **Publicar o motor de escuta Windows**: idem com a tag `motores-stt-windows-v1`
      (`publicar-motores-stt-windows.yml`). Observação: no Windows o WebView2 também NÃO tem
      Web Speech API — o motor é necessário lá também.
- [ ] Testar a revisão de pronúncia ponta a ponta com microfone real (gravação via sounddevice no
      sidecar + transcrição Paraformer): `bash builds/build_sidecars_stt_linux.sh` gera o bundle
      local em `python_backend/dist/paraformer_server/`, que o app já reconhece como instalado.

## Pendências

- [ ] Teste `TestObterQuestoesRevisaoTodosOsModos` em `wails_app/revisao_test.go:58` falha de forma intermitente ("modo geral: hanzi X repetido na sessão", com hanzi diferente a cada execução) — indica bug real de repetição na seleção aleatória de questões em `wails_app/revisao.go` (arquivo tem mudanças extensas ainda não commitadas). Não investigado nesta sessão (fora do escopo da renomeação da API do `overlay`); precisa de correção na lógica de sorteio/exclusão de repetidos.

## Pendências anteriores

- [ ] **Assinatura de código** dos binários dos sidecars (exige certificado; reduz falso positivo de
      antivírus). Hoje a integridade é garantida só por sha256.

### Armazenamento (ideias futuras)
- [ ] Permitir mover a pasta de modelos para outro disco.
- [ ] Limpeza automática agendada de logs antigos.
