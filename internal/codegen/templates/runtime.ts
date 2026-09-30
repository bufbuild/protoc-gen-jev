interface Rule {
  name: string;
  type: "choice" | "noul" | "score";
  instructions: string;
  field: string;
  kind: string;
  choices?: Record<string, string>;
  levels?: { value: number; description: string }[];
  threshold: number;
  oneof?: Record<string, string>;
  typed?: boolean;
}

function object(value: unknown, label: string): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label}: expected an object`);
  }
  return value as Record<string, unknown>;
}

function scoreQuestion(q: Rule) {
  const descriptions = (q.levels ?? []).map(l => l.description);
  if (descriptions.length < 2) throw new Error(`${q.name}: at least two score levels are required`);
  return score(q.instructions, [descriptions[0], descriptions[1], ...descriptions.slice(2)]);
}

function questions(rules: readonly Rule[]) {
  return Object.fromEntries(rules.map(q => [q.name,
    q.type === "choice" ? choice(q.instructions, q.choices ?? {}) :
    q.type === "score" ? scoreQuestion(q) :
    noul(q.instructions)
  ]));
}

function mapResponse(response: Record<string, unknown>, rules: readonly Rule[], responseField?: string, metaField?: string): JsonObject {
  const values: JsonObject = Object.create(null) as JsonObject;
  const canonical = Object.hasOwn(response, "answers");
  for (const q of rules) {
    const group = object(response[canonical ? "answers" : {choice:"choices", noul:"nouls", score:"scores"}[q.type]], q.name);
    if (!Object.hasOwn(group, q.name)) throw new Error(`Missing answer for ${q.name}`);
    const item = object(group[q.name], q.name);
    if ((canonical || Object.hasOwn(item, "type")) && item.type !== q.type) {
      throw new Error(`${q.name}: expected answer type ${q.type}`);
    }
    if (q.type === "choice") {
      const label = item.choice;
      if (typeof label !== "string" || !Object.hasOwn(q.choices ?? {}, label)) {
        throw new Error(`${q.name}: invalid choice`);
      }
      if (q.oneof) {
        if (q.oneof[label] === "bytes") {
          // Protobuf field identifiers are ASCII, so the selected label needs no Unicode conversion.
          values[label] = btoa(label);
        } else { values[label] = label; }
      } else if (q.typed) {
        const choiceObj: Record<string, unknown> = { value: label };
        if (typeof item.confidence === "number") choiceObj.confidence = item.confidence;
        if (item.probabilities && typeof item.probabilities === "object") choiceObj.probabilities = item.probabilities;
        values[q.field] = choiceObj as unknown as JsonValue;
      } else { values[q.field] = label; }
    } else if (q.type === "noul") {
      const value = Object.hasOwn(item, "noul") ? item.noul : item.result;
      let flag: boolean;
      let prob: number;
      if (typeof value === "boolean") {
        flag = value;
        prob = value ? 1.0 : 0.0;
      } else if (typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 1) {
        flag = value >= q.threshold;
        prob = value;
      } else {
        throw new Error(`${q.name}: invalid noul`);
      }
      if (q.typed) {
        const noulObj: Record<string, unknown> = { value: flag, probability: prob };
        if (typeof item.confidence === "number") noulObj.confidence = item.confidence;
        values[q.field] = noulObj as unknown as JsonValue;
      } else {
        values[q.field] = flag;
      }
    } else {
      const pos = item.score;
      const levels = q.levels ?? [];
      if (typeof pos !== "number" || !Number.isFinite(pos) || levels.length < 2 || pos < 0 || pos > levels.length - 1) {
        throw new Error(`${q.name}: invalid score`);
      }
      const i = Math.floor(pos);
      let value = levels[i].value;
      if (i + 1 < levels.length) value = (1 - (pos - i)) * value + (pos - i) * levels[i + 1].value;
      if (q.kind.startsWith("int") || q.kind.startsWith("uint")) {
        const magnitude = Math.abs(value);
        const whole = Math.floor(magnitude);
        value = Math.sign(value) * (whole + (magnitude - whole >= 0.5 ? 1 : 0));
      }
      if (!Number.isFinite(value)) throw new Error(`${q.name}: non-finite domain value`);
      if (q.typed) {
        const scoreObj: Record<string, unknown> = { value, score: pos };
        if (typeof item.confidence === "number") scoreObj.confidence = item.confidence;
        if (item.probabilities && typeof item.probabilities === "object") scoreObj.probabilities = item.probabilities;
        if (item.legend && typeof item.legend === "object") scoreObj.legend = item.legend;
        values[q.field] = scoreObj as unknown as JsonValue;
      } else {
        values[q.field] = q.kind === "int64" || q.kind === "uint64" ? value.toFixed(0) : q.kind === "float32" ? Math.fround(value) : value;
      }
    }
  }
  if (responseField) {
    if (!response.answers) {
      const answers: Record<string, unknown> = {};
      for (const key of ["choices", "nouls", "scores"]) {
        const grp = response[key];
        if (grp && typeof grp === "object") {
          Object.assign(answers, grp);
        }
      }
      if (Object.keys(answers).length > 0) {
        response.answers = answers;
      }
    }
    if (response.answers && typeof response.answers === "object") {
      for (const item of Object.values(response.answers as Record<string, unknown>)) {
        if (item && typeof item === "object") {
          const rec = item as Record<string, unknown>;
          if (typeof rec.noul === "boolean") rec.noul = rec.noul ? 1.0 : 0.0;
          if (typeof rec.result === "boolean") rec.noul = rec.result ? 1.0 : 0.0;
        }
      }
    }
    values[responseField] = response as unknown as JsonValue;
  }
  if (metaField) {
    const metaObj: Record<string, unknown> = {};
    if (typeof response.model === "string") metaObj.model = response.model;
    if (response.usage && typeof response.usage === "object") {
      const u = response.usage as Record<string, unknown>;
      const usageObj: Record<string, unknown> = {};
      if (typeof u.input_tokens === "number") usageObj.input_tokens = u.input_tokens;
      if (typeof u.output_tokens === "number") usageObj.output_tokens = u.output_tokens;
      metaObj.usage = usageObj;
    }
    values[metaField] = metaObj as unknown as JsonValue;
  }
  return values;
}

