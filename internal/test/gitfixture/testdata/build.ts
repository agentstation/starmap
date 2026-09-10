import { mkdirSync, copyFileSync, appendFileSync } from "node:fs";
mkdirSync("dist", { recursive: true });
copyFileSync("fixture.json", "dist/_api.json");
appendFileSync(process.env.STARMAP_GIT_BUILD_LOG!, "built\n");
