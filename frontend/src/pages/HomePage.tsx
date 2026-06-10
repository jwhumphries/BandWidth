import {useEffect, useState} from 'react';

type ServerStatus = 'checking' | 'ok' | 'unreachable';

export default function HomePage() {
  const [status, setStatus] = useState<ServerStatus>('checking');

  useEffect(() => {
    let cancelled = false;
    fetch('/healthz')
      .then(res => {
        if (!cancelled) setStatus(res.ok ? 'ok' : 'unreachable');
      })
      .catch(() => {
        if (!cancelled) setStatus('unreachable');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-5xl font-bold">BandWidth</h1>
          <p className="py-4">Practice tracking for musicians and bands.</p>
          <ServerStatusBadge status={status} />
        </div>
      </div>
    </main>
  );
}

function ServerStatusBadge({status}: {status: ServerStatus}) {
  if (status === 'checking') {
    return <span className="badge">Checking…</span>;
  }
  if (status === 'ok') {
    return <span className="badge badge-success">Server online</span>;
  }
  return <span className="badge badge-error">Server unreachable</span>;
}
