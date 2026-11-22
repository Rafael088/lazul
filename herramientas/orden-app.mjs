// Reescribe las tomas con el lado y el orden de manos que la app le da a Go
// (seguimiento.mjs). Uso: node herramientas/orden-app.mjs <tomas> <salida>
import fs from "node:fs";
import { identificar } from "../desktop/app/frontend/seguimiento.mjs";

const [de, a] = process.argv.slice(2);
if (!de || !a) {
  console.error("uso: node herramientas/orden-app.mjs <tomas> <salida>");
  process.exit(2);
}
fs.mkdirSync(a, { recursive: true });
for (const f of fs.readdirSync(de).filter((x) => x.endsWith(".jsonl"))) {
  const [cabecera, ...lineas] = fs.readFileSync(`${de}/${f}`, "utf8").trim().split("\n");
  const c = JSON.parse(cabecera);
  const aspecto = (c.ancho || 640) / (c.alto || 480);
  let conocidas = [];
  const salida = lineas.map((l) => {
    const cuadro = JSON.parse(l);
    const r = identificar(conocidas, cuadro.manos || [], cuadro.t, aspecto);
    conocidas = r.conocidas;
    return JSON.stringify({ ...cuadro, manos: r.manos });
  });
  fs.writeFileSync(`${a}/${f}`, [cabecera, ...salida].join("\n") + "\n");
}
