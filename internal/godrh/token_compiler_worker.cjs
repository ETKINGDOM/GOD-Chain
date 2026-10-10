// Embedded private offline worker. No dependency loading, filesystem imports,
// network callbacks, signing or deployment. This is not an OS security sandbox.
'use strict';
const fs = require('node:fs');
const crypto = require('node:crypto');
const vm = require('node:vm');
const limit = 48 * 1024 * 1024;
try {
  const chunks = [], chunk = Buffer.alloc(64 * 1024);
  let total = 0, count;
  while ((count = fs.readSync(0,chunk,0,chunk.length,null)) > 0) {
    total += count;
    if (total > limit) throw new Error();
    chunks.push(Buffer.from(chunk.subarray(0,count)));
  }
  const raw = Buffer.concat(chunks,total);
  if (raw.length === 0) throw new Error();
  const job = JSON.parse(raw.toString('utf8'));
  if (job.purpose !== 'GOD Chain private offline compiler job v1' || typeof job.compiler !== 'string' ||
      typeof job.input !== 'string' || typeof job.compilerSHA256 !== 'string') throw new Error();
  const bytes = Buffer.from(job.compiler, 'base64');
  if (!bytes.length || bytes.length > 32 * 1024 * 1024 || crypto.createHash('sha256').update(bytes).digest('hex') !== job.compilerSHA256) throw new Error();
  const source = new TextDecoder('utf-8', {fatal:true}).decode(bytes);
  // Only separately hash-checked known project compiler bytes reach this realm.
  // No process, require, fetch, source-reader or SMT callback is supplied.
  const realm = vm.createContext(Object.create(null), {codeGeneration:{strings:false,wasm:true}});
  vm.runInContext(source, realm, {timeout:15000});
  const version = vm.runInContext('Module.cwrap("solidity_version","string",[])()', realm, {timeout:1000});
  realm.input = job.input;
  const output = vm.runInContext('Module.cwrap("solidity_compile","string",["string","number","number"])(input,0,0)', realm, {timeout:30000});
  if (typeof output !== 'string' || Buffer.byteLength(output) > 8 * 1024 * 1024) throw new Error();
  const result = JSON.parse(output);
  if (result.errors !== undefined && !Array.isArray(result.errors)) throw new Error();
  let hasErrors = false;
  for (const diagnostic of result.errors || []) {
    if (!diagnostic || !['error','warning','info'].includes(diagnostic.severity)) throw new Error();
    hasErrors ||= diagnostic.severity === 'error';
  }
  let runtime = '', linkReferences = {}, immutableReferences = {};
  if (!hasErrors) {
    if (!result.contracts || !Object.hasOwn(result.contracts,job.targetSource) ||
        !Object.hasOwn(result.contracts[job.targetSource],job.targetContract)) throw new Error();
    const code = result.contracts[job.targetSource][job.targetContract]?.evm?.deployedBytecode;
    if (!code || typeof code.object !== 'string' || !code.linkReferences || !code.immutableReferences) throw new Error();
    runtime = code.object; linkReferences = code.linkReferences; immutableReferences = code.immutableReferences;
  }
  // No diagnostic, source, path, input, ABI or metadata is printed. The private
  // parent consumes this bounded code result and exposes only redacted flags.
  process.stdout.write(JSON.stringify({version,hasErrors,runtime,linkReferences,immutableReferences}));
} catch {
  process.stderr.write('GOD Chain offline compiler worker rejected\n');
  process.exitCode = 1;
}
