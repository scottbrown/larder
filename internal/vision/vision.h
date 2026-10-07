#ifndef LARDER_VISION_H
#define LARDER_VISION_H

// Renders every page of the PDF at path and runs Vision text recognition on
// it. Returns a malloc'd JSON document of the form
// [{"width":600,"height":2288,"observations":[{"t":"text","x":0.1,"y":0.9,
// "w":0.2,"h":0.01,"s":-0.05}, ...]}, ...] with one object per page,
// coordinates normalised to the page (origin bottom-left) and "s" the slope
// of the text baseline in pixels, or NULL with *err set to a malloc'd
// message.
char *larder_recognize_pdf(const char *path, double dpi, char **err);

#endif
