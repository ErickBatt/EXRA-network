'use client';

import { useState } from 'react';

type ProxyGuideProps = {
  apiKey: string;
  nodeAddress?: string;
  nodePort?: number;
};

export default function ProxyGuide({ apiKey, nodeAddress = 'api.exra.net', nodePort = 8080 }: ProxyGuideProps) {
  const [lang, setLang] = useState<'curl' | 'python' | 'node'>('curl');
  const [copied, setCopied] = useState(false);

  const proxyUrl = `http://buyer:${apiKey}@${nodeAddress}:${nodePort}`;

  const snippets = {
    curl: `curl -x ${proxyUrl} https://ifconfig.me`,
    python: `import requests\n\nproxies = {\n  "http": "${proxyUrl}",\n  "https": "${proxyUrl}",\n}\n\nresponse = requests.get("https://ifconfig.me", proxies=proxies)\nprint(response.text)`,
    node: `const axios = require('axios');\n\naxios.get('https://ifconfig.me', {\n  proxy: {\n    host: '${nodeAddress}',\n    port: ${nodePort},\n    auth: {\n      username: 'buyer',\n      password: '${apiKey}'\n    }\n  }\n}).then(res => console.log(res.data));`
  };

  const copy = () => {
    navigator.clipboard.writeText(snippets[lang]);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="proxy-guide-wrap">
      <div className="proxy-guide-tabs">
        {(['curl', 'python', 'node'] as const).map(l => (
          <button 
            key={l}
            className={`guide-tab ${lang === l ? 'active' : ''}`}
            onClick={() => setLang(l)}
          >
            {l}
          </button>
        ))}
      </div>
      <div className="proxy-code-box">
        <pre><code>{snippets[lang]}</code></pre>
        <button className="copy-btn-code" onClick={copy}>
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
      <div className="proxy-guide-hint">
        * Authentication uses your unique API Key as the password.
      </div>
    </div>
  );
}
