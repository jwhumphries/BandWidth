// Footer is a small, unobtrusive copyright link shown on every page,
// authenticated or not, since it sits in App.tsx below the route tree.
export default function Footer() {
  return (
    <div className="px-3 py-2 text-right">
      <a
        href="https://github.com/jwhumphries"
        target="_blank"
        rel="noopener noreferrer"
        className="text-base-content/30 hover:text-base-content/60 text-xs transition-colors"
      >
        © {new Date().getFullYear()} John Humphries
      </a>
    </div>
  );
}
