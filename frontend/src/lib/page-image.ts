export async function pageImage(file: File): Promise<string> {
  if (file.type && !file.type.startsWith('image/')) {
    throw new Error('Choose an image file')
  }
  if (file.size > 5 * 1024 * 1024) {
    throw new Error('Choose an image smaller than 5 MB')
  }
  const image = await createImageBitmap(file).catch(() => {
    throw new Error('Could not open this image. Try PNG, JPG, WebP or GIF.')
  })
  try {
    const canvas = document.createElement('canvas')
    canvas.width = 128
    canvas.height = 128
    const context = canvas.getContext('2d')!
    const scale = 128 / Math.max(image.width, image.height)
    const width = image.width * scale
    const height = image.height * scale
    context.drawImage(image, (128 - width) / 2, (128 - height) / 2, width, height)
    return canvas.toDataURL('image/png')
  } finally {
    image.close()
  }
}
