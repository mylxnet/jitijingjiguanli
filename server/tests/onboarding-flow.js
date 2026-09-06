// 引导页 7 步完整流程模拟
const http = require('http');
const req = (m,p,b) => new Promise(r=>{
  const q=http.request({hostname:'localhost',port:8080,path:p,method:m,headers:{'Content-Type':'application/json'}},res=>{
    let d='';res.on('data',x=>d+=x);res.on('end',()=>r({s:res.statusCode,j:JSON.parse(d)}));
  });
  if(b)q.write(JSON.stringify(b));q.end();
});

(async()=>{
  console.log('=== 新引导页 7 步完整模拟 ===\n');
  await req('POST','/api/auth/login',{account:'neworg',password:'123'});

  // Step 1: 加流转企业
  console.log('【Step 1】加流转企业（flow）');
  const flowParties = ['李志强','何德仁','金富贵'];
  for(const n of flowParties){
    const r = await req('POST','/api/parties',{name:n,types:['flow']});
    console.log('  POST party:', r.s, r.j.data?.name);
    for(const l1Name of ['土地流转费收入','流转管理费']){
      let cats = (await req('GET','/api/categories')).j.data;
      const l1 = cats.find(c=>c.name===l1Name);
      if(!l1.children.find(c=>c.name===n)){
        await req('POST','/api/categories',{name:n,level:2,parentId:l1.id,kind:'equity'});
        console.log('  → 自动在',l1Name,'下建 L2:',n);
      }
    }
  }
  console.log('  ✅ 完成');

  // Step 2: 流转企业 L2 余额
  console.log('\n【Step 2】填流转企业余额（10000 元）');
  let cats = (await req('GET','/api/categories')).j.data;
  for(const l1Name of ['土地流转费收入','流转管理费']){
    const l1 = cats.find(c=>c.name===l1Name);
    for(const l2 of l1.children){
      await req('PUT',`/api/categories/${l2.id}`,{openingBalanceCents:1000000});
    }
    console.log('  ',l1Name,'×',l1.children.length,'个 L2');
  }

  // Step 3: 投资公司
  console.log('\n【Step 3】加投资公司（invest）');
  const invParties = ['县乡村振兴发展集体有限公司','富民公司'];
  for(const n of invParties){
    await req('POST','/api/parties',{name:n,types:['invest']});
    let cats2 = (await req('GET','/api/categories')).j.data;
    const l1 = cats2.find(c=>c.name==='长期投资');
    if(!l1.children.find(c=>c.name===n)){
      await req('POST','/api/categories',{name:n,level:2,parentId:l1.id,kind:'equity'});
      console.log('  → 自动在长期投资下建 L2:',n);
    }
  }
  console.log('  ✅ 完成');

  // Step 4: 投资公司余额
  console.log('\n【Step 4】填投资公司余额（50000 元）');
  cats = (await req('GET','/api/categories')).j.data;
  const investL1 = cats.find(c=>c.name==='长期投资');
  for(const l2 of investL1.children){
    await req('PUT',`/api/categories/${l2.id}`,{openingBalanceCents:5000000});
  }
  console.log('  长期投资 ×',investL1.children.length,'个 L2');

  // Step 5-6: 再投资跳过
  console.log('\n【Step 5-6】再投资跳过');

  // Step 7: 其他设置
  console.log('\n【Step 7】银行 + 预置科目');
  await req('PUT','/api/settings',{bankBalanceCents:50000000});
  console.log('  银行存款期初: 500,000.00');
  cats = (await req('GET','/api/categories')).j.data;
  for(const l1 of cats){
    for(const l2 of l1.children){
      if(l2.preset && l2.name==='上级补助'){
        await req('PUT',`/api/categories/${l2.id}`,{openingBalanceCents:30000000});
        console.log('  上级补助期初: 300,000.00');
      }
    }
  }

  // === 最终验证 ===
  console.log('\n' + '='.repeat(50));
  console.log('📊 最终数据验证');
  console.log('='.repeat(50));

  const finalCats = (await req('GET','/api/categories')).j.data;
  const finalParties = (await req('GET','/api/parties')).j.data;
  const settings = (await req('GET','/api/settings')).j.data;

  // 验证点
  let allOk = true;
  const checks = [];

  // 1. 8 个 L1
  checks.push(['8 个 L1', finalCats.length === 8]);
  // 2. 长期投资有 2+ 示例 L2（原有 + 新建）
  const investCount = finalCats.find(c=>c.name==='长期投资').children.length;
  checks.push(['长期投资 ≥ 2 个 L2', investCount >= 2]);
  // 3. 土地流转费收入 ≥ 3 个（祥云示例 + 李志强/何德仁/金富贵）
  const rentCount = finalCats.find(c=>c.name==='土地流转费收入').children.length;
  checks.push(['土地流转费收入 ≥ 3 个 L2', rentCount >= 3]);
  // 4. 流转管理费 ≥ 3 个
  const feeCount = finalCats.find(c=>c.name==='流转管理费').children.length;
  checks.push(['流转管理费 ≥ 3 个 L2', feeCount >= 3]);
  // 5. 投资收益 L1 存在且空
  checks.push(['投资收益 L1 空', finalCats.find(c=>c.name==='投资收益').children.length === 0]);
  // 6. 再投资 L1 存在且空
  checks.push(['再投资 L1 空', finalCats.find(c=>c.name==='再投资').children.length === 0]);
  // 7. PARTIES ≥ 5 个
  checks.push(['PARTIES ≥ 5', finalParties.length >= 5]);
  // 8. 银行存款 = 50000000 分
  checks.push(['银行期初 = 500000', settings.data?.bankBalanceCents === 50000000]);

  console.log('\n✅ 验证结果:');
  for(const [label, ok] of checks){
    console.log(' ', ok ? '✅' : '❌', label);
    if(!ok) allOk = false;
  }

  // 打印完整科目树
  console.log('\n📁 完整科目结构:');
  for(const l1 of finalCats){
    console.log(`  ${l1.preset?'📌':' '} ${l1.name} (${l1.children.length} L2)`);
    for(const l2 of l1.children){
      console.log(`    ${l2.preset?'📌':' '} ${l2.name}  bal=${(l2.openingBalanceCents||0)/100}`);
    }
  }

  console.log('\n👥 往来单位:');
  for(const p of finalParties){
    console.log(`  ${p.name} → [${(p.types||[p.type]).join(',')}]`);
  }

  console.log('\n' + (allOk ? '🎉 全流程通过！' : '❌ 有失败项'));
})();
