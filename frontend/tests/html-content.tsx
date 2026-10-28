import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { HTMLContent } from '../src/components/html-content'

const messages = [
  '<p>First message</p><img src="https://tracker.invalid/pixel?message=first" onerror="window.unsafe=true"><script>window.unsafe=true</script><iframe src="https://tracker.invalid/frame"></iframe>',
  '<p>Second message</p><img src="https://tracker.invalid/pixel?message=second">',
]

function Fixture() {
  const [message, setMessage] = useState(0)
  return <><button id="next" onClick={() => setMessage(1)}>Next message</button><HTMLContent html={messages[message]} /></>
}

createRoot(document.getElementById('root')!).render(<Fixture />)
