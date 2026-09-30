import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/_app/')({
  beforeLoad: () => {
    throw redirect({ to: '/o/$object', params: { object: 'people' }, replace: true })
  },
})
