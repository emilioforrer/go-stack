import { usePage } from '@inertiajs/react';

type PageProps = {
  text: string;
};

export default function Home() {
  const { text } = usePage().props as PageProps;

  return (
    <div style={{ fontFamily: 'sans-serif', padding: '2rem' }}>
      <h1>Home</h1>
      <p>{text}</p>
    </div>
  );
}
