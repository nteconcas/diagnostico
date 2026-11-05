package report

// tplHTML é o modelo do relatório em HTML autossuficiente (imagens embutidas em base64).
const tplHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Relatório de Diagnóstico — {{.EscolaNome}}</title>
<style>
  :root { --azul:#1d4ed8; --azul-esc:#1e3a8a; --verde:#15803d; --tinta:#0f172a; --cinza:#475569; --linha:#e2e8f0; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: "Segoe UI", Arial, sans-serif; color: var(--tinta); background:#f1f5f9; line-height:1.55; }
  .folha { max-width: 900px; margin: 0 auto; background:#fff; }
  .capa { padding: 0 0 40px; text-align: center; background:#fff; }
  .faixa { background: linear-gradient(90deg, var(--azul), #2563eb); color:#fff; padding: 26px 30px; text-align:left; border-bottom: 5px solid var(--verde); }
  .faixa h1 { font-size: 24px; letter-spacing:.3px; }
  .faixa p { opacity:.9; font-size: 13px; margin-top: 4px; }
  .capa h2 { font-size: 28px; margin: 30px 30px 4px; color: var(--azul-esc); }
  .capa .nte { color: var(--cinza); font-size: 15px; margin-bottom: 18px; }
  .capa img.fachada { width: 92%; max-width: 720px; aspect-ratio: 4 / 3; object-fit: contain; border: 1px solid var(--linha); border-radius: 8px; background: #fff; }
  .meta { display:flex; flex-wrap: wrap; gap: 12px; justify-content:center; margin: 26px 30px 0; }
  .meta div { background:#f8fafc; border:1px solid var(--linha); border-radius: 10px; padding: 10px 16px; font-size: 13.5px; }
  .meta b { color: var(--cinza); display:block; font-size: 11.5px; text-transform: uppercase; letter-spacing:.5px; }
  .secao { padding: 26px 30px 6px; break-inside: avoid-page; }
  .secao h3 { font-size: 19px; color: var(--azul-esc); border-left: 5px solid var(--azul); padding-left: 12px; margin-bottom: 14px; }
  .secao h3 small { color: var(--verde); font-size: 12px; display:block; letter-spacing:.6px; text-transform:uppercase; }
  table { width: 100%; border-collapse: collapse; margin: 8px 0 14px; font-size: 14px; }
  th { background: #dbeafe; color: var(--azul-esc); text-align: left; }
  th, td { border: 1px solid var(--linha); padding: 8px 12px; }
  tr:nth-child(even) td { background: #f8fafc; }
  .kv { display:grid; grid-template-columns: 190px 1fr; gap: 6px 14px; font-size: 14.5px; margin: 6px 0 14px; }
  .kv b { color: var(--cinza); font-weight: 600; }
  .barra { background:#e2e8f0; border-radius: 999px; height: 14px; overflow:hidden; display:flex; margin: 6px 0 4px; }
  .barra span { height: 100%; }
  .barra .azul { background: var(--azul); } .barra .verde { background: var(--verde); }
  .legenda { font-size: 12.5px; color: var(--cinza); margin-bottom: 12px; }
  .galeria { display:flex; flex-wrap: wrap; gap: 10px; margin: 10px 0 16px; }
  .galeria figure { break-inside: avoid; }
  .galeria img { width: 200px; height: 150px; object-fit: contain; background:#fff; border-radius: 8px; border:1px solid var(--linha); }
  .galeria figcaption { font-size: 11.5px; color: var(--cinza); text-align:center; margin-top:3px; max-width: 200px; }
  .lab, .espaco, .projeto { border: 1px solid var(--linha); border-radius: 12px; padding: 14px 18px; margin: 10px 0 14px; break-inside: avoid; }
  .lab h4, .espaco h4, .projeto h4 { color: var(--verde); font-size: 15.5px; margin-bottom: 4px; }
  .lab p, .projeto p, .espaco p { font-size: 14px; color: var(--tinta); white-space: pre-wrap; }
  .chips { display:flex; flex-wrap:wrap; gap:8px; }
  .chip { background:#ecfdf5; color:#166534; border:1px solid #bbf7d0; padding: 5px 14px; border-radius: 999px; font-size: 13.5px; }
  a { color: var(--azul); }
  .rodape { text-align:center; color: var(--cinza); font-size: 12.5px; padding: 26px; border-top: 1px solid var(--linha); margin-top: 20px; }
  .btn-print { position: fixed; top: 14px; right: 16px; background: var(--azul); color:#fff; border:0; border-radius: 999px; padding: 10px 22px; font-size: 14px; cursor:pointer; box-shadow: 0 6px 18px rgba(29,78,216,.35); }
  @media print {
    body { background:#fff; }
    .btn-print { display: none; }
    .secao { break-inside: avoid; }
    .faixa { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
</style>
</head>
<body>
<button class="btn-print" onclick="window.print()">🖨 Imprimir / Salvar PDF</button>
<div class="folha">
  <div class="faixa">
    <h1>Formulário de Diagnóstico Escolar</h1>
    <p>Servidores · Infraestrutura · Espaços · Projetos · Ofertas de cursos — uso institucional da rede educacional</p>
  </div>
  <div class="capa">
    <h2>{{.EscolaNome}}</h2>
    <p class="nte">NTE {{.NTE}}{{if .Responsavel}} · Responsável pelo preenchimento: {{.Responsavel}}{{end}}</p>
    {{if .Fachada}}<img class="fachada" src="data:image/jpeg;base64,{{b64 .Fachada.Caminho}}" alt="Fachada da escola">{{end}}
    <div class="meta">
      <div><b>Status</b>{{if eq .Status "enviado"}}Enviado{{else}}Rascunho{{end}}{{if .EnviadoEm}} — {{.EnviadoEm}}{{end}}</div>
      <div><b>Total de servidores</b>{{.TotalServ}}</div>
      <div><b>Gerado em</b>{{.GeradoEm}}{{if .GeradoPor}} · por {{.GeradoPor}}{{end}}</div>
    </div>
  </div>

  <div class="secao">
    <h3><small>Identificação e localização</small>Endereço, contatos e GPS</h3>
    <div class="kv">
      <b>Endereço</b><span>{{.Endereco}}</span>
      {{if .GPS.Capturado}}<b>Localização GPS</b><span>{{printf "%.6f" .GPS.Lat}}, {{printf "%.6f" .GPS.Lng}} — <a href="{{.GPSLink}}" target="_blank">abrir no mapa</a></span>{{end}}
      {{if .GestorFoto}}<b>Gestor(a)</b><span><img src="data:image/jpeg;base64,{{b64 .GestorFoto.Caminho}}" alt="Gestor" style="height:90px;border-radius:10px;border:1px solid var(--linha)"></span>{{end}}
    </div>
    {{if .Contatos}}
    <table>
      <tr><th>Setor</th><th>Telefone</th><th>E-mail</th></tr>
      {{range .Contatos}}<tr><td>{{.Setor}}</td><td>{{.Telefone}}</td><td>{{.Email}}</td></tr>{{end}}
    </table>
    {{end}}
  </div>

  <div class="secao">
    <h3><small>Seção 1</small>Servidores</h3>
    <table>
      <tr><th>Categoria</th><th>Quantidade</th></tr>
      <tr><td>Servidores na gestão (direção, vice-direção, secretaria e afins)</td><td>{{.Servidores.Gestao}}</td></tr>
      <tr><td>Servidores no pedagógico (coordenação, supervisão, orientação e apoio)</td><td>{{.Servidores.Pedagogico}}</td></tr>
      <tr><td>Professores efetivos (concurso / quadro permanente)</td><td>{{.Servidores.Efetivos}}</td></tr>
      <tr><td>Professores ACT (admitidos em caráter temporário)</td><td>{{.Servidores.ACT}}</td></tr>
      <tr><th>Total de servidores</th><th>{{.TotalServ}}</th></tr>
    </table>
    <div class="barra"><span class="azul" style="width:{{printf "%.1f" .PctEfetivos}}%"></span><span class="verde" style="width:{{printf "%.1f" .PctACT}}%"></span></div>
    <p class="legenda">Distribuição dos docentes: {{printf "%.0f" .PctEfetivos}}% efetivos ({{.Servidores.Efetivos}}) · {{printf "%.0f" .PctACT}}% ACT ({{.Servidores.ACT}})</p>
  </div>

  <div class="secao">
    <h3><small>Seção 2</small>Infraestrutura</h3>
    <div class="kv">
      <b>Salas de aula</b><span>{{.Salas.Total}} no total · {{.Salas.Ocupadas}} ocupadas ({{printf "%.0f" .OcupacaoPct}}% de ocupação)</span>
      <b>Equipamentos tecnológicos</b><span>Total: {{.TotalEquip}} — {{.Equip.Computadores}} computadores, {{.Equip.Tablets}} tablets, {{.Equip.Projetores}} projetores, {{.Equip.Telas}} telas interativas</span>
      {{if .Equip.Outros}}<b>Outros equipamentos</b><span>{{.Equip.Outros}}</span>{{end}}
    </div>
    {{if .LabsFixos}}{{range .LabsFixos}}
    <div class="lab"><h4>{{.Nome}}</h4><p>{{.Descricao}}</p>{{galeria .Fotos}}</div>{{end}}{{end}}
    {{if .LabsOutros}}{{range .LabsOutros}}
    <div class="lab"><h4>{{.Nome}}</h4><p>{{.Descricao}}</p>{{galeria .Fotos}}</div>{{end}}{{end}}
    <h4 style="margin:14px 0 4px;color:var(--cinza);font-size:14px">Registro fotográfico — antes da gestão 2023/2027</h4>
    {{galeria .FotosAntes}}
    <h4 style="margin:14px 0 4px;color:var(--cinza);font-size:14px">Registro fotográfico — gestão atual 2023/2027</h4>
    {{galeria .FotosAtual}}
  </div>

  {{if .Espacos}}
  <div class="secao">
    <h3><small>Seção 3</small>Espaços da escola</h3>
    {{range .Espacos}}<div class="espaco"><h4>{{.Nome}}</h4>{{galeria .Fotos}}</div>{{end}}
  </div>
  {{end}}

  {{if .Projetos}}
  <div class="secao">
    <h3><small>Seção 4</small>Projetos</h3>
    {{range .Projetos}}
    <div class="projeto"><h4>{{.Nome}}</h4><p>{{.Descricao}}</p>{{galeria .Fotos}}</div>{{end}}
  </div>
  {{end}}

  {{if .Cursos}}
  <div class="secao">
    <h3><small>Seção 5</small>Ofertas de cursos</h3>
    <div class="chips">{{range .Cursos}}<span class="chip">{{.}}</span>{{end}}</div>
  </div>
  {{end}}

  <div class="rodape">Formulário de Diagnóstico Escolar · Uso institucional da rede educacional · Gerado em {{.GeradoEm}}</div>
</div>
</body>
</html>`
