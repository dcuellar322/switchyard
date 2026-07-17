import { generateLLMS } from './generate-llms'
import { generateReference } from './generate-reference'

await generateReference()
await generateLLMS()
