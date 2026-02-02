import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/teste/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/teste/"!</div>
}
