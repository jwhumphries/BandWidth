import {AudioLines} from 'lucide-react';
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate} from 'react-router';
import {useSignup} from '../hooks/auth';

export default function SignupPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const navigate = useNavigate();
  const signup = useSignup();

  const submit = (e: FormEvent) => {
    e.preventDefault();
    signup.mutate(
      {username, email, password},
      {onSuccess: () => void navigate('/')},
    );
  };

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <div className="flex flex-col items-center gap-3">
          <span className="bg-primary text-primary-content grid size-14 place-items-center rounded-box shadow-lg">
            <AudioLines className="size-8" strokeWidth={2.25} />
          </span>
          <h1 className="font-display text-4xl font-bold tracking-tight">
            Band<span className="text-primary">Width</span>
          </h1>
        </div>
        <form
          className="card bg-base-100 border-base-300/60 w-full border p-6 shadow-xl"
          onSubmit={submit}
        >
          <fieldset className="fieldset">
            <label className="label" htmlFor="username">
              Username
            </label>
            <input
              id="username"
              className="input w-full"
              value={username}
              onChange={e => setUsername(e.target.value)}
              required
            />
            <label className="label" htmlFor="email">
              Email
            </label>
            <input
              id="email"
              type="email"
              className="input w-full"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
            />
            <label className="label" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="input w-full"
              value={password}
              onChange={e => setPassword(e.target.value)}
              minLength={8}
              required
            />
            {signup.error && (
              <div role="alert" className="alert alert-error mt-2">
                {signup.error.message}
              </div>
            )}
            <button
              className="btn btn-primary mt-4"
              disabled={signup.isPending}
            >
              Sign up
            </button>
          </fieldset>
        </form>
        <p className="text-sm">
          Already have an account?{' '}
          <Link className="link" to="/login">
            Log in
          </Link>
        </p>
      </div>
    </main>
  );
}
