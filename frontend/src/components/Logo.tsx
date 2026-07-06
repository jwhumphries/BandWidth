import type {SVGProps} from 'react';

export default function Logo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <g
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
        opacity={0.4}
      >
        <path d="M2 10v3" />
        <path d="M6 6v11" />
        <path d="M10 3v18" />
        <path d="M14 8v7" />
        <path d="M18 5v13" />
        <path d="M22 10v3" />
      </g>
      <g fill="currentColor">
        <path d="M2.67 5 L12 12 L2.67 19 Z" />
        <path d="M12 5 L21.33 12 L12 19 Z" />
      </g>
    </svg>
  );
}
