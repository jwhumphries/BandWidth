import {Mail, Plus, Shield, Trash2, Users} from 'lucide-react';
import {useState} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../components/songs/ConfirmModal';
import {
  useAccessPolicy,
  useAddAllowedEmail,
  useAdminBands,
  useAdminUsers,
  useDeleteAdminBand,
  useDeleteAdminUser,
  useRemoveAllowedEmail,
  useSetAccessPolicy,
} from '../hooks/admin';

type Tab = 'users' | 'bands' | 'access';

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>('users');

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 className="font-display text-3xl font-bold tracking-tight">Admin</h1>
      <div role="tablist" className="tabs tabs-boxed w-fit">
        <button
          role="tab"
          className={`tab ${tab === 'users' ? 'tab-active' : ''}`}
          onClick={() => setTab('users')}
        >
          Users
        </button>
        <button
          role="tab"
          className={`tab ${tab === 'bands' ? 'tab-active' : ''}`}
          onClick={() => setTab('bands')}
        >
          Bands
        </button>
        <button
          role="tab"
          className={`tab ${tab === 'access' ? 'tab-active' : ''}`}
          onClick={() => setTab('access')}
        >
          Access policy
        </button>
      </div>
      {tab === 'users' && <AdminUsersPanel />}
      {tab === 'bands' && <AdminBandsPanel />}
      {tab === 'access' && <AdminAccessPolicyPanel />}
    </div>
  );
}

function AdminUsersPanel() {
  const {data: users = []} = useAdminUsers();
  const deleteUser = useDeleteAdminUser();
  const [target, setTarget] = useState<{id: number; username: string} | null>(
    null,
  );

  return (
    <section className="flex flex-col gap-2">
      {users.length === 0 ? (
        <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
          <Users className="text-base-content/30 size-10" />
          <p>No users yet.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {users.map(u => (
            <li
              key={u.id}
              className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-4"
            >
              <span className="min-w-0 flex-1 truncate font-display font-semibold">
                {u.username}
              </span>
              <span className="text-base-content/55 truncate text-sm">
                {u.email}
              </span>
              <button
                className="btn btn-error btn-outline btn-sm gap-1.5"
                aria-label={`Delete ${u.username}`}
                onClick={() => setTarget({id: u.id, username: u.username})}
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {deleteUser.error && (
        <div role="alert" className="alert alert-error">
          {deleteUser.error.message}
        </div>
      )}
      <ConfirmModal
        open={target !== null}
        title="Delete user"
        message={
          target
            ? `Delete "${target.username}" and everything they own, including any band they created?`
            : ''
        }
        confirmLabel="Delete"
        onConfirm={() => {
          if (target) deleteUser.mutate(target.id);
          setTarget(null);
        }}
        onCancel={() => setTarget(null)}
      />
    </section>
  );
}

function AdminBandsPanel() {
  const {data: bands = []} = useAdminBands();
  const deleteBand = useDeleteAdminBand();
  const [target, setTarget] = useState<{id: number; name: string} | null>(null);

  return (
    <section className="flex flex-col gap-2">
      {bands.length === 0 ? (
        <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
          <Shield className="text-base-content/30 size-10" />
          <p>No bands yet.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {bands.map(b => (
            <li
              key={b.id}
              className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-4"
            >
              <span className="min-w-0 flex-1 truncate font-display font-semibold">
                {b.name}
              </span>
              <span className="text-base-content/55 text-sm">
                created by {b.creatorUsername}
              </span>
              <span className="text-base-content/55 font-mono text-xs">
                {b.memberCount} {b.memberCount === 1 ? 'member' : 'members'}
              </span>
              <button
                className="btn btn-error btn-outline btn-sm gap-1.5"
                aria-label={`Delete ${b.name}`}
                onClick={() => setTarget({id: b.id, name: b.name})}
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {deleteBand.error && (
        <div role="alert" className="alert alert-error">
          {deleteBand.error.message}
        </div>
      )}
      <ConfirmModal
        open={target !== null}
        title="Delete band"
        message={target ? `Delete "${target.name}" for every member?` : ''}
        confirmLabel="Delete"
        onConfirm={() => {
          if (target) deleteBand.mutate(target.id);
          setTarget(null);
        }}
        onCancel={() => setTarget(null)}
      />
    </section>
  );
}

function AdminAccessPolicyPanel() {
  const {data: policy} = useAccessPolicy();
  const setPolicy = useSetAccessPolicy();
  const addEmail = useAddAllowedEmail();
  const removeEmail = useRemoveAllowedEmail();
  const [email, setEmail] = useState('');

  const submitEmail = (e: FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;
    addEmail.mutate({email: email.trim()}, {onSuccess: () => setEmail('')});
  };

  return (
    <section className="flex flex-col gap-4">
      <label className="flex items-center gap-3">
        <input
          type="checkbox"
          className="toggle toggle-primary"
          checked={policy?.enabled ?? false}
          onChange={e => setPolicy.mutate({enabled: e.target.checked})}
        />
        <span>
          {policy?.enabled
            ? 'Registration restricted to the allow-list below'
            : 'Registration open to anyone'}
        </span>
      </label>

      <ul className="flex flex-col gap-2">
        {(policy?.allowedEmails ?? []).map(entry => (
          <li
            key={entry.id}
            className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-3"
          >
            <Mail className="text-base-content/40 size-4" />
            <span className="min-w-0 flex-1 truncate">{entry.email}</span>
            <button
              className="btn btn-ghost btn-sm btn-square"
              aria-label={`Remove ${entry.email}`}
              onClick={() => removeEmail.mutate(entry.id)}
            >
              <Trash2 className="size-4" />
            </button>
          </li>
        ))}
      </ul>

      <form className="flex gap-2" onSubmit={submitEmail}>
        <input
          type="email"
          className="input min-w-0 flex-1"
          placeholder="friend@example.com"
          value={email}
          onChange={e => setEmail(e.target.value)}
        />
        <button
          className="btn btn-primary gap-1.5"
          disabled={addEmail.isPending}
        >
          <Plus className="size-4" />
          Add
        </button>
      </form>
      {(addEmail.error ?? removeEmail.error) && (
        <div role="alert" className="alert alert-error">
          {(addEmail.error ?? removeEmail.error)?.message}
        </div>
      )}
    </section>
  );
}
