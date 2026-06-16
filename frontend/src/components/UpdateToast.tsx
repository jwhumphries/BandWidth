import {useRegisterSW} from 'virtual:pwa-register/react';

// UpdateToast surfaces the service-worker "new version ready" event as a
// small toast. With registerType: 'prompt', needRefresh becomes true when a
// new SW is waiting; clicking Reload activates it and reloads the page.
export default function UpdateToast() {
  const {
    needRefresh: [needRefresh],
    updateServiceWorker,
  } = useRegisterSW();

  if (!needRefresh) {
    return null;
  }

  return (
    <div className="toast toast-end z-50">
      <div className="alert alert-info">
        <span>New version available.</span>
        <button
          className="btn btn-sm"
          onClick={() => void updateServiceWorker(true)}
        >
          Reload
        </button>
      </div>
    </div>
  );
}
