import {useState} from 'react';
import type {FormEvent} from 'react';
import {useCreateResource, useDeleteResource} from '../../hooks/songs';
import type {Resource} from '../../lib/types';

export default function ResourceList({
  songId,
  resources,
}: {
  songId: number;
  resources: Resource[];
}) {
  const createResource = useCreateResource(songId);
  const deleteResource = useDeleteResource(songId);
  const [url, setUrl] = useState('');
  const [label, setLabel] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    createResource.mutate(
      {url, label},
      {
        onSuccess: () => {
          setUrl('');
          setLabel('');
        },
      },
    );
  };

  return (
    <div className="flex flex-col gap-2">
      {resources.length === 0 && (
        <p className="text-base-content/60 text-sm">
          No links yet — add tabs, videos, or tutorials.
        </p>
      )}
      <ul className="flex flex-col gap-1">
        {resources.map(r => (
          <li key={r.id} className="flex items-center gap-2">
            <a
              className="link min-w-0 flex-1 truncate"
              href={r.url}
              target="_blank"
              rel="noreferrer noopener"
            >
              {r.label ? (
                <>
                  {r.label}
                  <span className="text-base-content/50 ml-1 text-xs">
                    {r.url}
                  </span>
                </>
              ) : (
                r.url
              )}
            </a>
            <button
              className="btn btn-ghost btn-xs"
              aria-label={`Remove ${r.label || r.url}`}
              onClick={() => deleteResource.mutate(r.id)}
            >
              ✕
            </button>
          </li>
        ))}
      </ul>
      <form className="flex flex-wrap gap-2" onSubmit={submit}>
        <input
          className="input input-sm min-w-0 flex-1"
          placeholder="https://…"
          value={url}
          onChange={e => setUrl(e.target.value)}
          required
        />
        <input
          className="input input-sm w-32"
          placeholder="Label"
          value={label}
          onChange={e => setLabel(e.target.value)}
        />
        <button className="btn btn-sm" disabled={createResource.isPending}>
          Add link
        </button>
      </form>
      {createResource.error && (
        <div role="alert" className="alert alert-error">
          {createResource.error.message}
        </div>
      )}
    </div>
  );
}
