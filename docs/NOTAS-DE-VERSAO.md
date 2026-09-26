# Guia de Notas de Versão (Changelog no App)

Este guia orienta pessoas desenvolvedoras e agentes de IA sobre como escrever, estruturar e publicar notas de versão no Hanzi Tracker.

---

## 1. Por que as notas moram no projeto?

Diferente de projetos tradicionais onde o changelog é editado manualmente na página de releases do GitHub:

- **Mesmo commit da alteração:** A nota nasce junto com o código que ela descreve.
- **Embutidas no binário:** Os arquivos de notas ficam embutidos no executável (`//go:embed notas`), garantindo disponibilidade imediata sem chamadas de rede ou dependência de APIs externas.
- **Publicação contínua no canal Dev:** Cada push na branch `main` gera uma release do canal Dev automaticamente. Como não há intervenção humana na esteira de CI/CD, as notas precisam viajar empacotadas no próprio commit.
- **Sem condições de corrida:** Evita divergências entre a compilação da release e a edição posterior do texto.

---

## 2. Quando escrever uma nota?

- **Escreva quando:** Houver qualquer alteração perceptível pelo usuário final — novas funcionalidades, melhorias visuais, mudanças de comportamento na interface ou correções de bugs observáveis.
- **Não escreva quando:** A mudança for estritamente interna (refatorações, otimizações sem impacto visual/funcional perceptível, ajustes em CI, correção de typos em comentários ou testes).
- **Momento:** Sempre inclua o arquivo da nota no **mesmo commit** ou no mesmo PR da alteração.

---

## 3. Contratos de Formato e Nomenclatura

### Nomenclatura de Arquivos (Contrato C1)

Todas as notas residem no diretório: `wails_app/notasversao/notas/`.

- **Nota principal em pt-BR (obrigatória):**  
  `AAAA-MM-DD-slug.md`  
  *Exemplo:* `2026-09-11-atualizacao-automatica.md`

- **Traduções opcionais:**  
  `AAAA-MM-DD-slug.<idioma>.md`  
  Onde `<idioma>` é um código suportado (atualmente `en` ou `es`).  
  *Exemplos:*  
  `2026-09-11-atualizacao-automatica.en.md`  
  `2026-09-11-atualizacao-automatica.es.md`

> ⚠️ **Atenção:**
> - O identificador único da nota é `AAAA-MM-DD-slug`.
> - **Nunca renomeie um arquivo de nota já publicado!** O identificador é gravado no estado local dos usuários; renomear o arquivo faz a nota reaparecer como novidade para todo mundo.
> - Uma tradução só pode existir se o arquivo base correspondente em pt-BR existir.
> - Se uma tradução não for fornecida para determinado idioma, o app usará automaticamente o texto em pt-BR como fallback.

### Formato do Conteúdo (Contrato C2)

O aplicativo utiliza um parser restrito de Markdown (texto puro) e não depende de bibliotecas externas pesadas no frontend.

#### Regras:
1. **Título:** A primeira linha com conteúdo deve ser `# <título>`. Só pode haver um título principal por arquivo.
2. **Grupos:** Linhas iniciadas com `## <subtítulo>` abrem um grupo temático (ex.: `## Novidades`, `## Correções`).
3. **Itens:** Linhas iniciadas com `- <texto>` definem os tópicos dentro do grupo corrente. (Itens declarados antes de qualquer `##` pertencem a um grupo sem subtítulo).
4. **Continuação de linha:** Linhas iniciadas com espaço ou tabulação logo após um item continuam o texto daquele item.
5. **Formatação proibida:** **Não utilize negrito (`**`) nem crases (`` ` ``)**. O parser rejeitará qualquer arquivo que contenha esses caracteres para preservar o design system limpo do aplicativo.
6. **Linhas em branco e CRLF:** São ignoradas automaticamente pelo parser.

#### Exemplo de arquivo de nota:

```markdown
# Atualização automática e novidades

## Novidades
- O app agora se atualiza sozinho ao abrir, pelo canal Estável ou Dev.
- Configurações → Info mostra a versão instalada e tem o botão "Verificar agora".

## Correções
- As escolhas feitas no instalador passam a valer na primeira abertura.
```

---

## 4. Como funciona no aplicativo (Contrato C3)

- **Primeira abertura pós-atualização:** Se houver notas de versão cujo identificador ainda não consta como visualizado, um pop-up é exibido automaticamente ao abrir o aplicativo.
- **Fechamento e persistência:** Ao fechar o pop-up (pelo botão "Entendi", pelo "×" ou clicando fora), todos os identificadores exibidos são salvos localmente em `notas_vistas.json` dentro da pasta de dados do usuário (`armazenamento.PastaDados()`).
- **Primeira execução com a feature:** sem `notas_vistas.json` (instalação nova, ou vinda de uma versão que ainda não tinha notas), todas as notas embutidas são marcadas como vistas e nada é mostrado — o usuário não recebe o histórico inteiro de uma vez. Por isso quem instala a primeira versão com notas só as encontra no histórico.
- **Builds locais e pré-visualização:** builds locais (`wails dev` ou execução local) também mostram o pop-up. Para rever uma nota, tire o id dela da lista `vistas` em `notas_vistas.json` (`%APPDATA%\HanziTracker\` no Windows, `~/.config/HanziTracker/` no Linux) e reabra o app. Apagar o arquivo não serve: sem ele, o app entende que é a primeira execução e marca tudo como visto.
- **Histórico completo:** Usuários podem consultar o histórico de todas as novidades a qualquer momento acessando **Configurações → Info → Novidades**.

---

## 5. Validação Automática

A integridade sintática e de nomenclatura de todas as notas é validada automaticamente pela suíte de testes do Go:

```powershell
cd wails_app
go test -vet=off ./notasversao/
```

Esse teste roda na CI em todo push e pull request (`testes.yml`) e também nos workflows de publicação (`publicar-app-windows.yml` e `publicar-app-linux.yml`), antes do build: uma nota com formatação inválida, nome fora do padrão ou data incorreta impede a publicação da release.
