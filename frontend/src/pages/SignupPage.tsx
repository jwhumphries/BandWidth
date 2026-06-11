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
        <h1 className="text-4xl font-bold">BandWidth</h1>
        <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
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
