# 💜 FinancesGo

Aplicação local (Go + SQLite) que junta **dois módulos integrados**:

1. **Financeiro** — saldo, contas fixas recorrentes, receitas, gastos avulsos e um motor de projeção que responde *"quanto posso gastar hoje / amanhã / esta semana / até o próximo salário"*.
2. **Pokémon GO** — catálogo de contas para venda (nível, time, raridades, preço com taxa GGMAX calculada automaticamente), com fluxo de venda/reembolso que entra sozinho no financeiro depois do período de espera.

Tudo em **um único binário**, com interface web em tema **preto + roxo**, rodando em `http://localhost:8080`. Sem instalador, sem Electron, sem servidor de banco separado.

---

## ✨ Funcionalidades

### Financeiro
- Saldo inicial configurável e **saldo total em destaque** no dashboard.
- **Contas fixas recorrentes** (nome, valor, dia de vencimento, categoria) com status visual: 🟢 paga · 🟡 a vencer · 🔴 atrasada.
- **Receitas recorrentes** (ex.: salário) que são lançadas automaticamente no dia certo, e **receitas / gastos avulsos**.
- **Motor de projeção** ("quanto posso gastar") em linha do tempo, considerando saldo atual + receitas previstas − contas a pagar no período.
- **Gráfico de fluxo de caixa** (entradas × saídas) dos últimos meses e filtros por mês e categoria.
- **Alertas** de contas a vencer nos próximos dias.

### Pokémon GO
- Cadastro completo: email, level, time (com a cor aplicada no card), descrição, lendários, shinys, quantidade de pokémons / capacidade da bag, valor base e **tags**.
- **Taxa GGMAX** configurável (padrão 15,98%) e **preço final calculado automaticamente** (`valor base × 1,1598`).
- **Botão de problema** → marca a conta de vermelho, registra data e observação.
- **Fluxo de venda**: ao vender, o valor **não** entra no saldo imediatamente; após *N* dias (padrão 7) uma rotina interna diária credita o valor no financeiro como receita "Venda Pokémon GO".
- **Reembolso**: reverte a venda; se o valor já havia entrado, lança uma saída automática para manter o histórico rastreável.
- **Simulador "e se eu vender agora?"** mostrando o impacto no saldo.
- **Histórico de preço** por conta.

### Relatórios, exportação e backup
- **Relatório mensal**: total vendido, ticket médio, contas com problema, gastos por categoria (gráfico), resultado do mês.
- **Exportação** das contas para **.xlsx** e **.csv**, com **histórico de exportações** dentro do app.
- **Backup / restauração** do banco: criar backup local, exportar o `.db` ou **importar** outro `.db` (troca de PC / restauração) — sem mexer em pastas do sistema.
- **Autenticação opcional** por senha local (recomendada por serem dados financeiros).

---

## 🚀 Como rodar

Pré-requisito: **Go 1.25+** (o driver SQLite é *puro Go* — não precisa de CGO nem de compilador C).

```bash
# clonar e entrar na pasta
git clone https://github.com/Matensy/FinancesGo.git
cd FinancesGo

# rodar direto
go run .

# ou compilar um binário único
go build -o financesgo .
./financesgo
```

Abra **http://localhost:8080** no navegador.

### Opções

```bash
./financesgo -addr :9000        # trocar a porta
./financesgo -data /caminho/dir # trocar a pasta de dados
```

Também via variáveis de ambiente: `FINANCESGO_ADDR`, `FINANCESGO_DATA`.

### Onde ficam os dados

Tudo dentro da pasta `./data` (criada na primeira execução):

```
data/
├── dados.db        # o banco SQLite — É o banco, não há processo separado
├── exports/        # planilhas .xlsx / .csv exportadas
└── backups/        # backups do banco
```

Para **trocar de PC**, copie a pasta `data/` (ou só o `dados.db`) para a outra máquina — todo o histórico vai junto. Ou use os botões **Exportar / Importar banco** em *Configurações*.

> ⚠️ Não abra o mesmo `dados.db` em duas instâncias ao mesmo tempo (ex.: pasta sincronizada por Drive/OneDrive aberta em dois PCs). SQLite não foi feito para escrita concorrente entre processos. Use um de cada vez.

---

## 🧱 Arquitetura

```
main.go                     ponto de entrada / flags
internal/
├── config/                 resolução de caminhos e porta
├── database/               conexão SQLite + schema (embed)
├── models/                 tipos de domínio
├── store/                  camada de acesso a dados (CRUD, queries)
├── finance/                motor de saldo e projeção
├── scheduler/              rotina diária (matura vendas, lança recorrentes)
├── export/                 geração de .xlsx / .csv
├── backup/                 snapshot/restore do banco (VACUUM INTO)
├── server/                 HTTP, sessões, handlers, templates
└── web/                    templates HTML + CSS/JS (embed)
```

- **Backend:** Go com a biblioteca padrão `net/http` (roteamento por padrões método+rota).
- **Banco:** SQLite via `modernc.org/sqlite` (puro Go).
- **Frontend:** templates Go + HTMX + Chart.js (assets *embutidos* no binário — funciona offline) + CSS próprio no tema preto/roxo.
- **Agendador:** rotina interna que roda ao iniciar e de hora em hora (operações idempotentes).

Migrar para nuvem depois (Fly.io, Railway, Oracle Free Tier) é simples: a lógica não muda, só onde o binário roda. Trocar SQLite por PostgreSQL exige adaptar apenas a camada `store`.

---

## 🧪 Testes

```bash
go test ./...
```

Cobrem o cálculo de saldo, pagamento de contas, cálculo da taxa GGMAX, maturação de vendas após o período de espera, reembolso (com e sem crédito prévio) e materialização de receitas recorrentes.
