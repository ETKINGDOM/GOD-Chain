// Read-only browser RPC and unsigned request preparation. No signer or proxy.
import {QueryUnavailableError,transientHTTP,isQueryUnavailable} from './query-errors.mjs';
const reads=new Set(['god_network','god_liveness','god_account','god_delegation','god_transaction','god_blocks','god_block','god_transactionDetails','eth_getBlockByNumber','eth_getTransactionReceipt']);
const fail=()=>new Error('Testnet request unavailable. Check the reviewed network and input.');
const uint=/^(0|[1-9][0-9]*)$/;
const evm=/^0x[0-9a-fA-F]{40}$/;
const native=/^god1[023456789acdefghjklmnpqrstuvwxyz]{20,80}$/;
const validator=/^godvaloper1[023456789acdefghjklmnpqrstuvwxyz]{20,80}$/;
const digest=/^[0-9a-f]{64}$/;
export function endpointURL(value){
  let u;try{u=new URL(value);}catch{throw fail();}
  if(u.username||u.password||u.search||u.hash||u.pathname!=='/'||value.includes('?')||value.includes('#')||
    (u.protocol!=='https:'&&!(u.protocol==='http:'&&['localhost','127.0.0.1','[::1]'].includes(u.hostname))))throw fail();
  return u.href;
}
export function createRPC(endpoint,fetcher=globalThis.fetch){
  const url=endpointURL(endpoint);let sequence=0;
  return async function read(method,params=[]){
    if(!reads.has(method)||!Array.isArray(params)||params.length>3)throw fail();
    const id=++sequence;
    const body=JSON.stringify({jsonrpc:'2.0',id,method,params});if(body.length>16384)throw fail();
    const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),5000);
    try{
      let response;
      try{response=await fetcher(url,{method:'POST',headers:{'Content-Type':'application/json'},body,signal:controller.signal,credentials:'omit',cache:'no-store',redirect:'error',referrerPolicy:'no-referrer'});}
      catch(error){if(controller.signal.aborted||error instanceof TypeError)throw new QueryUnavailableError();throw error;}
      if(transientHTTP(response.status))throw new QueryUnavailableError();
      if(!response.ok)throw fail();
      let length=0,raw='';const decoder=new TextDecoder('utf-8',{fatal:true});
      const reader=response.body?.getReader();
      if(!reader)throw fail();
      try{for(;;){let chunk;try{chunk=await reader.read();}catch{throw new QueryUnavailableError();}const {value,done}=chunk;if(done)break;length+=value.byteLength;if(length>2*1024*1024){await reader.cancel();throw fail();}raw+=decoder.decode(value,{stream:true});}raw+=decoder.decode();}finally{reader.releaseLock();}
      const value=JSON.parse(raw);
      if(value?.jsonrpc!=='2.0'||value.id!==id||value.error||!Object.hasOwn(value,'result'))throw fail();
      return value.result;
    }catch(error){if(isQueryUnavailable(error))throw error;throw fail();}finally{clearTimeout(timer);}
  };
}
export function checkNetwork(n){
  if(n?.commit?.synthetic!==true||n.commit.realAssets!==false||typeof n.chainId!=='string'||!/^god-test-[0-9a-f]{32}$/.test(n.chainId)||
    typeof n.evmChainId!=='string'||!uint.test(n.evmChainId)||BigInt(n.evmChainId)<1n||BigInt(n.evmChainId)>18446744073709551615n||n.decimals!==18||
    n.totalGod!=='1000000000000000000000000000'||typeof n.commit.height!=='string'||!uint.test(n.commit.height)||BigInt(n.commit.height)<1n)throw fail();
  for(const field of ['rewardPoolGod','pendingRewardGod','outstandingG','pendingG'])if(typeof n[field]!=='string'||n[field].length>90||!uint.test(n[field]))throw fail();
  return n;
}
export function units(value){
  if(typeof value!=='string'||value.length>90||!/^(0|[1-9][0-9]*)(\.[0-9]{1,18})?$/.test(value))throw fail();
  const [whole,fraction='']=value.split('.');const amount=BigInt(whole)*10n**18n+BigInt(fraction.padEnd(18,'0'));
  if(amount<=0n||amount>2n**256n-1n)throw fail();return amount.toString();
}
export function displayUnits(value){
  if(typeof value!=='string'||value.length>90||!uint.test(value))throw fail();
  const amount=BigInt(value);const fraction=(amount%10n**18n).toString().padStart(18,'0').replace(/0+$/,'');
  return (amount/10n**18n).toString()+(fraction?'.'+fraction:'');
}
export function fundingRequest(network,recipient,pin,amount){
  checkNetwork(network);
  if(!evm.test(recipient)||!digest.test(pin))throw fail();
  const value=units(amount);if(BigInt(value)>5n*10n**18n)throw fail();
  return {version:1,mode:'synthetic',kind:'test-funding',chainId:network.chainId,evmChainId:network.evmChainId,bundleSha256:pin,recipient,amountSmallestUnits:value};
}
export function nativeRequest(network,account,input){
  checkNetwork(network);
  if(account?.commit?.synthetic!==true||account.commit.realAssets!==false||account.moduleAccount!==false||account.exists!==true||
    typeof account.nativeAddress!=='string'||!native.test(account.nativeAddress)||typeof account.accountNumber!=='string'||!uint.test(account.accountNumber)||typeof account.sequence!=='string'||!uint.test(account.sequence)||
    typeof input.gas!=='string'||!uint.test(input.gas)||BigInt(input.gas)<1n||BigInt(input.gas)>2000000n||typeof input.fee!=='string'||!uint.test(input.fee)||BigInt(input.fee)<BigInt(input.gas)||BigInt(input.fee)>2n**256n-1n)throw fail();
  const q={version:1,mode:'synthetic',chainId:network.chainId,accountNumber:account.accountNumber,sequence:account.sequence,gas:input.gas,feeSmallestUnits:input.fee,action:input.action,validator:'',recipient:'',amountSmallestUnits:'',minGodOutSmallestUnits:'',deadlineUnixNanos:''};
  if(!['delegate','undelegate','claim-g','transfer-g','donate-god','redeem-g'].includes(q.action))throw fail();
  if(q.action!=='claim-g')q.amountSmallestUnits=units(input.amount);
  if(['delegate','undelegate'].includes(q.action)){if(!validator.test(input.validator))throw fail();q.validator=input.validator;}
  if(['delegate','undelegate','redeem-g'].includes(q.action)&&BigInt(q.amountSmallestUnits)<10n**18n)throw fail();
  if(['transfer-g','redeem-g'].includes(q.action)){if(!native.test(input.recipient))throw fail();q.recipient=input.recipient;}
  if(q.action==='redeem-g'){
    q.minGodOutSmallestUnits=units(input.minimum);
    if(!/^\d{4}-\d\d-\d\dT\d\d:\d\d(:\d\d(\.\d{1,3})?)?(Z|[+-]\d\d:\d\d)$/.test(input.deadline))throw fail();
    const milliseconds=Date.parse(input.deadline);
    if(!Number.isSafeInteger(milliseconds)||milliseconds<=Date.now())throw fail();
    const nanos=BigInt(milliseconds)*1000000n;if(nanos>9223372036854775807n)throw fail();q.deadlineUnixNanos=nanos.toString();
  }
  return q;
}
export function feedbackRequest(category,steps){
  if(!['connection','transfer','staking','rewards','interface','other'].includes(category)||typeof steps!=='string'||steps.length<1||steps.length>2000||
    /https?:\/\/|0x[0-9a-fA-F]{40,64}|-----BEGIN|\b(?:password|private.?key|mnemonic|seed.?phrase)\s*[:=]/i.test(steps))throw fail();
  return {version:1,mode:'synthetic',kind:'test-feedback',category,reproductionSteps:steps};
}
export function blockParams(value){
  if(value==='latest')return ['latest',false];if(!uint.test(value)||BigInt(value)<1n||BigInt(value)>9223372036854775807n)throw fail();
  return ['0x'+BigInt(value).toString(16),false];
}
export function transactionParams(hash){if(!/^0x[0-9a-fA-F]{64}$/.test(hash))throw fail();return [hash];}

export function transferRequest(network,account,pin,recipient,amount,price){
  checkNetwork(network);
  if(!digest.test(pin)||(!native.test(recipient)&&!evm.test(recipient))||account?.commit?.synthetic!==true||account.commit.realAssets!==false||account.exists!==true||account.moduleAccount!==false||!native.test(account.nativeAddress)||
    typeof account.sequence!=='string'||!uint.test(account.sequence)||BigInt(account.sequence)>=18446744073709551615n||typeof price!=='string'||price.length>78||!uint.test(price)||BigInt(price)<1n||BigInt(price)*21000n>10n**16n)throw fail();
  return {version:1,mode:'synthetic',kind:'god-transfer',chainId:network.chainId,evmChainId:network.evmChainId,bundleSha256:pin,sender:account.nativeAddress,sequence:account.sequence,recipient,amountSmallestUnits:units(amount),gasPriceSmallestUnits:price};
}

// Render only known public summary fields, never arbitrary memo/calldata/logs.
export function publicSummary(value){
  if(value?.synthetic!==true||value.realAssets!==false||!['native','ethereum'].includes(value.kind)||!/^0x[0-9a-fA-F]{64}$/.test(value.consensusHash)||
    typeof value.height!=='string'||!uint.test(value.height)||BigInt(value.height)<1n||typeof value.consensusIndex!=='string'||!uint.test(value.consensusIndex)||typeof value.sdkSuccessful!=='boolean'||!Number.isSafeInteger(value.code)||value.code<0||value.code>4294967295)throw fail();
  const result={};
  for(const key of ['kind','consensusHash','ethereumHash','height','blockHash','consensusIndex','code','sdkSuccessful','gasWanted','gasUsed','sender','senderEVM','recipient','valueSmallestUnits','inputBytes','sequence','evmExecution','fee','synthetic','realAssets']){
    if(!Object.hasOwn(value,key))continue;const v=value[key];if(v!==null&&!['string','number','boolean'].includes(typeof v)||typeof v==='string'&&v.length>200)throw fail();result[key]=v;
  }
  if(value.kind==='ethereum'&&(!/^0x[0-9a-fA-F]{64}$/.test(value.ethereumHash)||!['succeeded','failed','sdk-failed'].includes(value.evmExecution)))throw fail();
  if(value.kind==='native'){
    if(!Array.isArray(value.operations)||value.operations.length>64)throw fail();
    result.operations=value.operations.map(operation=>{const out={};for(const key of ['operation','sender','recipient','validator','amountSmallestUnits','minGodOutSmallestUnits','deadlineUnixNanos'])if(Object.hasOwn(operation,key)){if(typeof operation[key]!=='string'||operation[key].length>200)throw fail();out[key]=operation[key];}return out;});
  }
  return result;
}
