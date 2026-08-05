// sudoku-flow SPA. F-11: ALL response data (solution digits, technique tags, witness cells,
// status, metrics, grade, catalog puzzles) and the echoed input are written to the DOM via
// textContent / createTextNode / element-property assignment ONLY. There is no innerHTML,
// insertAdjacentHTML, or document.write anywhere in this file. Grep it for "innerHTML" — zero.
"use strict";

const CELLS = 81;
const EMPTY = "0";

// ADR-0002 technique ladder, cheapest-first, and its Easy/Medium/Hard/Expert bands (mirrors
// internal/solver/ladder.go). Used to order the histogram and name the hardest technique.
const LADDER = [
  "naked_single", "hidden_single",
  "locked_candidates_pointing", "locked_candidates_claiming", "naked_subset", "hidden_subset",
  "x_wing", "swordfish", "jellyfish", "xy_wing",
  "xyz_wing", "w_wing", "simple_colouring",
];
const BAND = {
  naked_single: "easy", hidden_single: "easy",
  locked_candidates_pointing: "medium", locked_candidates_claiming: "medium",
  naked_subset: "medium", hidden_subset: "medium",
  x_wing: "hard", swordfish: "hard", jellyfish: "hard", xy_wing: "hard",
  xyz_wing: "expert", w_wing: "expert", simple_colouring: "expert",
};

// --- grid construction (symmetric seams) -------------------------------------------------

const gridEl = document.getElementById("grid");
const inputs = [];

for (let i = 0; i < CELLS; i++) {
  const row = Math.floor(i / 9);
  const col = i % 9;
  const cell = document.createElement("input");
  cell.className = "cell";
  cell.type = "text";
  cell.inputMode = "numeric";
  cell.maxLength = 1;
  cell.setAttribute("aria-label", `Row ${row + 1} column ${col + 1}`);
  // Every cell paints its own right+bottom interior line; outer top/left and the box seams
  // (cols 2,5,8 / rows 2,5,8) thicken to 2px — four even edges, even internal seams.
  if (row === 0) cell.classList.add("edge-top");
  if (col === 0) cell.classList.add("edge-left");
  if (col === 2 || col === 5 || col === 8) cell.classList.add("seam-right");
  if (row === 2 || row === 5 || row === 8) cell.classList.add("seam-bottom");
  cell.addEventListener("input", onCellInput);
  cell.addEventListener("paste", onPaste);
  inputs.push(cell);
  gridEl.appendChild(cell);
}

// Only 1-9 stays; anything else clears the cell. Editing a cell drops solved/step styling and
// returns the grid to editable state.
function onCellInput(e) {
  const v = e.target.value.replace(/[^1-9]/g, "");
  e.target.value = v;
  clearCellStyles(e.target);
  e.target.removeAttribute("readonly");
  exitStepMode();
}

function onPaste(e) {
  const text = (e.clipboardData || window.clipboardData).getData("text") || "";
  const cleaned = text.replace(/\s+/g, "");
  if (cleaned.length >= CELLS) {
    e.preventDefault();
    loadPuzzle(cleaned.slice(0, CELLS));
  }
}

function clearCellStyles(cell) {
  cell.classList.remove("solved", "hl-place", "hl-witness", "hl-elim");
}

// loadPuzzle populates the grid and returns it to the editable state.
function loadPuzzle(s) {
  exitStepMode();
  for (let i = 0; i < CELLS; i++) {
    const ch = s[i];
    const cell = inputs[i];
    clearCellStyles(cell);
    cell.removeAttribute("readonly");
    cell.value = ch >= "1" && ch <= "9" ? ch : "";
  }
  resultEl.hidden = true;
  setStatus("", false);
}

function readGrid() {
  let out = "";
  for (let i = 0; i < CELLS; i++) {
    const v = inputs[i].value;
    out += v >= "1" && v <= "9" ? v : EMPTY;
  }
  return out;
}

// --- puzzle catalog dropdown -------------------------------------------------------------

const selectEl = document.getElementById("puzzle-select");
let currentTier = ""; // catalog section the loaded puzzle came from, if any

async function loadCatalog() {
  try {
    const resp = await fetch("/v1/puzzles");
    if (!resp.ok) return;
    const data = await resp.json();
    for (const sec of Array.isArray(data.sections) ? data.sections : []) {
      const name = typeof sec.name === "string" ? sec.name : "";
      const puzzles = Array.isArray(sec.puzzles) ? sec.puzzles : [];
      const group = document.createElement("optgroup");
      group.label = name; // property assignment.
      puzzles.forEach((p, idx) => {
        const opt = document.createElement("option");
        opt.value = p;
        opt.textContent = `${name} #${idx + 1}`; // textContent.
        opt.dataset.tier = name;
        group.appendChild(opt);
      });
      selectEl.appendChild(group);
    }
  } catch (_) {
    /* dropdown stays empty; typing/paste still work */
  }
}

selectEl.addEventListener("change", () => {
  const opt = selectEl.selectedOptions[0];
  if (!opt || !opt.value) return;
  loadPuzzle(opt.value);
  currentTier = opt.dataset.tier || "";
});

// --- controls ----------------------------------------------------------------------------

const solveBtn = document.getElementById("solve");
const clearBtn = document.getElementById("clear");
const statusEl = document.getElementById("status");
const resultEl = document.getElementById("result");
const metricsEl = document.getElementById("metrics");
const histEl = document.getElementById("hist");
const logEl = document.getElementById("log");

clearBtn.addEventListener("click", () => {
  selectEl.value = "";
  currentTier = "";
  loadPuzzle(EMPTY.repeat(CELLS));
});

solveBtn.addEventListener("click", solve);

function setStatus(msg, isErr) {
  statusEl.textContent = msg; // textContent: never innerHTML.
  statusEl.classList.toggle("err", !!isErr);
}

async function solve() {
  const puzzle = readGrid();
  solveBtn.disabled = true;
  setStatus("Solving…", false);
  resultEl.hidden = true;
  try {
    const resp = await fetch("/v1/solve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ puzzle }),
    });
    const data = await resp.json();
    if (!resp.ok) {
      setStatus(data && data.error ? data.error : `Request failed (${resp.status})`, true);
      return;
    }
    render(puzzle, data);
  } catch (err) {
    setStatus("Network error: " + String(err && err.message ? err.message : err), true);
  } finally {
    solveBtn.disabled = false;
  }
}

// --- rendering (F-11: textContent / createTextNode / property assignment only) ------------

let solveData = null; // last /v1/solve response
let baseInput = "";   // the puzzle string sent to /v1/solve (givens reference)

function render(input, data) {
  setStatus(data.solved ? "Solved" : `Status: ${data.status}`, !data.solved);
  solveData = data;
  baseInput = input;
  paintStats(data);
  paintHist(Array.isArray(data.events) ? data.events : []);
  buildLog(Array.isArray(data.events) ? data.events : []);
  resultEl.hidden = false;
  goToStep((data.events || []).length); // reveal the full solve; user can step back
}

// --- statistics --------------------------------------------------------------------------

function paintStats(data) {
  metricsEl.replaceChildren();
  const events = Array.isArray(data.events) ? data.events : [];

  let placements = 0, eliminations = 0;
  for (const ev of events) {
    if (ev.placement) placements++;
    if (Array.isArray(ev.eliminations)) eliminations += ev.eliminations.length;
  }
  let givens = 0;
  for (const ch of baseInput) if (ch >= "1" && ch <= "9") givens++;

  const tiles = [];
  tiles.push(["difficulty", data.grade || "—"]);
  if (currentTier) tiles.push(["catalog tier", currentTier]);
  tiles.push(["hardest step", prettyTech(hardestTech(events))]);
  tiles.push(["events", numOr(data.eventCount)]);
  tiles.push(["placements", placements]);
  if (eliminations) tiles.push(["eliminations", eliminations]);
  tiles.push(["givens", givens]);
  tiles.push(["iterations", numOr(data.iterations)]);
  tiles.push(["candidate checks", numOr(data.candidateChecks)]);
  if (placements > 0 && typeof data.candidateChecks === "number") {
    tiles.push(["checks / placement", Math.round(data.candidateChecks / placements)]);
  }
  tiles.push(["solve time (ms)", fmtMs(data.solveTimeMs)]);

  for (const [label, value] of tiles) metricsEl.appendChild(metric(label, value));
}

function metric(label, value) {
  const wrap = document.createElement("div");
  wrap.className = "metric";
  const k = document.createElement("span");
  k.className = "k";
  k.textContent = label;
  const v = document.createElement("span");
  v.className = "v";
  v.textContent = String(value ?? ""); // textContent.
  wrap.append(k, v);
  return wrap;
}

function paintHist(events) {
  histEl.replaceChildren();
  const counts = new Map();
  for (const ev of events) {
    const t = typeof ev.technique === "string" ? ev.technique : "?";
    counts.set(t, (counts.get(t) || 0) + 1);
  }
  const techs = [...counts.keys()].sort((a, b) => ladderIndex(a) - ladderIndex(b));
  let max = 1;
  for (const c of counts.values()) if (c > max) max = c;

  for (const t of techs) {
    const n = counts.get(t);
    const row = document.createElement("div");
    row.className = "hrow";

    const name = document.createElement("span");
    name.className = "hname";
    name.textContent = prettyTech(t); // textContent.

    const track = document.createElement("div");
    track.className = "hbar-track";
    const bar = document.createElement("div");
    bar.className = "hbar " + (BAND[t] || "");
    bar.style.width = Math.max(2, Math.round((n / max) * 100)) + "%"; // style property, not markup.
    track.appendChild(bar);

    const count = document.createElement("span");
    count.className = "hcount";
    count.textContent = String(n);

    row.append(name, track, count);
    histEl.appendChild(row);
  }
}

// --- event log + stepper -----------------------------------------------------------------

let logRows = [];   // <li> per event, index-aligned with events
let stepIndex = 0;  // 0 = initial givens; k = state after events[k-1]
let playTimer = null;

const posEl = document.getElementById("s-pos");
const descEl = document.getElementById("s-desc");
const firstBtn = document.getElementById("s-first");
const prevBtn = document.getElementById("s-prev");
const playBtn = document.getElementById("s-play");
const nextBtn = document.getElementById("s-next");
const lastBtn = document.getElementById("s-last");
const explainBody = document.getElementById("explain-body");

firstBtn.addEventListener("click", () => { stopPlay(); goToStep(0); });
prevBtn.addEventListener("click", () => { stopPlay(); goToStep(stepIndex - 1); });
nextBtn.addEventListener("click", () => { stopPlay(); goToStep(stepIndex + 1); });
lastBtn.addEventListener("click", () => { stopPlay(); goToStep(eventCount()); });
playBtn.addEventListener("click", togglePlay);

function eventCount() { return solveData && Array.isArray(solveData.events) ? solveData.events.length : 0; }

function buildLog(events) {
  logEl.replaceChildren();
  logRows = [];
  events.forEach((ev, idx) => {
    const li = logRow(ev);
    li.addEventListener("click", () => { stopPlay(); goToStep(idx + 1); });
    logRows.push(li);
    logEl.appendChild(li);
  });
}

function logRow(ev) {
  const li = document.createElement("li");
  const tech = document.createElement("span");
  tech.className = "tech";
  tech.textContent = prettyTech(ev.technique); // technique tag via textContent.

  const eff = effectText(ev);
  const witness = witnessText(ev.witnessCells);

  li.append(document.createTextNode(`#${numOr(ev.seq)} `), tech);
  if (eff) li.appendChild(document.createTextNode(" — " + eff));
  if (witness) {
    const w = document.createElement("span");
    w.className = "witness";
    w.textContent = "  witness: " + witness; // witness cells via textContent.
    li.appendChild(w);
  }
  return li;
}

// goToStep repaints the grid to the state after `k` events (0 = givens only), highlights the
// active event's cells, updates the description + position, and marks the active log row.
function goToStep(k) {
  const events = solveData && Array.isArray(solveData.events) ? solveData.events : [];
  stepIndex = Math.max(0, Math.min(k, events.length));

  if (stepIndex === 0) {
    paintState(baseInput, null);
    setDesc(null);
    paintExplain(null);
  } else {
    const ev = events[stepIndex - 1];
    paintState(typeof ev.gridAfter === "string" ? ev.gridAfter : baseInput, activeOf(ev));
    setDesc(ev);
    paintExplain(ev);
  }

  posEl.textContent = `Step ${stepIndex} / ${events.length}`;
  logRows.forEach((li, idx) => li.classList.toggle("active", idx === stepIndex - 1));
  if (stepIndex > 0 && logRows[stepIndex - 1]) {
    logRows[stepIndex - 1].scrollIntoView({ block: "nearest" });
  }
  firstBtn.disabled = prevBtn.disabled = stepIndex === 0;
  nextBtn.disabled = lastBtn.disabled = stepIndex === events.length;
}

// Plain-English definitions of each ladder technique (mirrors internal/solver/ladder.go), shown
// in the Explanation window and updated by goToStep as the user walks the solve.
const TECHNIQUES = {
  naked_single: "The cell has only one candidate left — every other digit already appears in its row, column, or box — so that digit must go here.",
  hidden_single: "Within a row, column, or box this digit can legally go in only one cell (even though that cell still holds other candidates), so it is placed there.",
  locked_candidates_pointing: "Inside one box, every remaining spot for a digit lines up in a single row or column. That digit can then be removed from the rest of that row/column outside the box.",
  locked_candidates_claiming: "Inside one row or column, every remaining spot for a digit falls within a single box. That digit can then be removed from the other cells of that box.",
  naked_subset: "A group of N cells in a unit together hold only N candidate digits (a naked pair/triple/quad). Those digits can be removed from the unit's other cells.",
  hidden_subset: "N digits are confined to the same N cells of a unit. Every other candidate can be removed from those N cells, exposing the subset.",
  x_wing: "A digit's only candidates in two rows sit in the same two columns, forming a rectangle. The digit can then be eliminated from those two columns everywhere else (and symmetrically for columns vs rows).",
  swordfish: "An X-wing scaled up to three rows and three columns: the digit is confined to three columns across three rows, so it is eliminated from those columns elsewhere.",
  jellyfish: "An X-wing scaled up to four rows and four columns — the same rectangle logic applied to a 4×4 pattern.",
  xy_wing: "Three bi-value cells form a hinge: a pivot 'XY' with two pincers 'XZ' and 'YZ'. Any cell seen by both pincers cannot be Z, so Z is eliminated there.",
  xyz_wing: "Like an XY-wing, but the pivot also contains Z (three cells over digits X/Y/Z). A cell seen by all three loses Z.",
  w_wing: "Two cells holding the identical two candidates are joined by a strong link on one of them; the other digit is then eliminated from any cell that sees both.",
  simple_colouring: "One digit's conjugate-pair chain is two-coloured. Where the colouring forces a contradiction (two of one colour in a unit, or a cell seeing both colours), the digit is eliminated.",
};

// paintExplain writes the plain-English definition of the current step's technique (or a hint
// when no step is active). F-11: textContent / createElement only.
function paintExplain(ev) {
  explainBody.replaceChildren();
  if (!ev) {
    const p = document.createElement("p");
    p.className = "explain-hint";
    p.textContent = "Solve a puzzle and step through it — each technique is explained here as you go.";
    explainBody.appendChild(p);
    return;
  }
  const t = ev.technique;
  const head = document.createElement("div");
  head.className = "explain-head";
  const name = document.createElement("span");
  name.className = "explain-name";
  name.textContent = prettyTech(t);
  head.appendChild(name);
  const band = BAND[t];
  if (band) {
    const chip = document.createElement("span");
    chip.className = "chip " + band;
    chip.textContent = band; // easy | medium | hard | expert
    head.appendChild(chip);
  }
  const def = document.createElement("p");
  def.className = "explain-def";
  def.textContent = TECHNIQUES[t] || "A constructive placement or elimination step in the solver's ladder.";
  explainBody.append(head, def);
}

// paintState renders an 81-char grid string. Givens (from baseInput) keep the ink colour;
// solver-filled cells render in the accent; `active` cells get the step highlight. All cells
// are readonly while a solved result is on screen.
function paintState(gridStr, active) {
  for (let i = 0; i < CELLS; i++) {
    const cell = inputs[i];
    const given = baseInput[i] >= "1" && baseInput[i] <= "9";
    const ch = gridStr[i];
    const filled = ch >= "1" && ch <= "9";
    cell.value = filled ? ch : ""; // property assignment.
    cell.setAttribute("readonly", "readonly");
    clearCellStyles(cell);
    cell.classList.toggle("solved", filled && !given);
    if (active) {
      if (active.placed === i) cell.classList.add("hl-place");
      else if (active.witness.has(i)) cell.classList.add("hl-witness");
      if (active.elim.has(i)) cell.classList.add("hl-elim");
    }
  }
}

function activeOf(ev) {
  const witness = new Set();
  if (Array.isArray(ev.witnessCells)) {
    for (const c of ev.witnessCells) if (c) witness.add(c.row * 9 + c.col);
  }
  const elim = new Set();
  if (Array.isArray(ev.eliminations)) {
    for (const e of ev.eliminations) if (e && e.cell) elim.add(e.cell.row * 9 + e.cell.col);
  }
  let placed = null;
  if (ev.placement && ev.placement.cell) placed = ev.placement.cell.row * 9 + ev.placement.cell.col;
  return { placed, witness, elim };
}

function setDesc(ev) {
  descEl.replaceChildren();
  if (!ev) {
    descEl.appendChild(document.createTextNode("Initial puzzle — givens only."));
    return;
  }
  const tech = document.createElement("span");
  tech.className = "tech";
  tech.textContent = prettyTech(ev.technique);
  descEl.append(document.createTextNode(`Step ${numOr(ev.seq)}: `), tech);
  const eff = effectText(ev);
  if (eff) descEl.appendChild(document.createTextNode(" — " + eff));
  const witness = witnessText(ev.witnessCells);
  if (witness) {
    const w = document.createElement("span");
    w.className = "witness";
    w.textContent = "  (witness: " + witness + ")";
    descEl.appendChild(w);
  }
}

function togglePlay() {
  if (playTimer) { stopPlay(); return; }
  if (stepIndex >= eventCount()) goToStep(0); // replay from the start
  playBtn.textContent = "Pause";
  playTimer = setInterval(() => {
    if (stepIndex >= eventCount()) { stopPlay(); return; }
    goToStep(stepIndex + 1);
  }, 350);
}

function stopPlay() {
  if (playTimer) { clearInterval(playTimer); playTimer = null; }
  playBtn.textContent = "Play";
}

function exitStepMode() {
  stopPlay();
}

// --- small formatters --------------------------------------------------------------------

function effectText(ev) {
  if (ev.placement && ev.placement.cell) {
    return `place ${numOr(ev.placement.value)} at ${cellName(ev.placement.cell)}`;
  }
  if (Array.isArray(ev.eliminations) && ev.eliminations.length > 0) {
    const parts = ev.eliminations.map((e) => `${numOr(e.candidate)}@${cellName(e.cell)}`);
    return "eliminate " + parts.join(", ");
  }
  return "";
}

function witnessText(cells) {
  if (!Array.isArray(cells) || cells.length === 0) return "";
  return cells.map(cellName).join(", ");
}

function cellName(c) {
  if (!c) return "?";
  return `r${numOr(c.row)}c${numOr(c.col)}`;
}

function prettyTech(t) {
  if (typeof t !== "string" || t === "") return "?";
  return t.replace(/_/g, " ").replace(/^./, (ch) => ch.toUpperCase());
}

function ladderIndex(t) {
  const i = LADDER.indexOf(t);
  return i === -1 ? LADDER.length : i;
}

function hardestTech(events) {
  let best = "";
  let bestIdx = -1;
  for (const ev of events) {
    const i = ladderIndex(ev.technique);
    if (i > bestIdx && LADDER.indexOf(ev.technique) !== -1) { bestIdx = i; best = ev.technique; }
  }
  return best;
}

function fmtMs(v) {
  if (typeof v !== "number") return "—";
  return v < 1 ? v.toFixed(4) : v.toFixed(2);
}

function numOr(n) {
  return typeof n === "number" ? String(n) : "?";
}

// --- init --------------------------------------------------------------------------------

paintExplain(null);
loadCatalog();
