/*
 * LD_PRELOAD shim for xfreerdp 2.x, built and loaded by the xfreerdp wrapper.
 *
 * A tiling compositor marks every tiled X11 window _NET_WM_STATE_MAXIMIZED_*.
 * In RemoteApp mode xfreerdp forwards that to the server as a maximize, so the
 * remote window covers the whole multi-monitor desktop while the local one is
 * a tile: clicks land offset and incremental repaints are dropped. Hiding the
 * two atoms from xfreerdp's own reads leaves the tile size as the only signal,
 * which it already forwards as an ordinary move/resize.
 */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>

typedef int (*get_property_fn)(Display *, Window, Atom, long, long, Bool, Atom, Atom *, int *,
                               unsigned long *, unsigned long *, unsigned char **);

int XGetWindowProperty(Display *dpy, Window w, Atom property, long offset, long length,
                       Bool del, Atom req_type, Atom *actual_type, int *actual_format,
                       unsigned long *nitems, unsigned long *bytes_after, unsigned char **prop)
{
	static get_property_fn real;
	if (!real)
		real = (get_property_fn)dlsym(RTLD_NEXT, "XGetWindowProperty");
	int rc = real(dpy, w, property, offset, length, del, req_type, actual_type,
	              actual_format, nitems, bytes_after, prop);
	if (rc != Success || !*prop || *actual_format != 32 || *actual_type != XA_ATOM ||
	    property != XInternAtom(dpy, "_NET_WM_STATE", True))
		return rc;

	Atom vert = XInternAtom(dpy, "_NET_WM_STATE_MAXIMIZED_VERT", True);
	Atom horz = XInternAtom(dpy, "_NET_WM_STATE_MAXIMIZED_HORZ", True);
	/* Xlib returns format-32 properties as an array of long. */
	unsigned long *atoms = (unsigned long *)*prop;
	unsigned long kept = 0;
	for (unsigned long i = 0; i < *nitems; i++)
		if (atoms[i] != vert && atoms[i] != horz)
			atoms[kept++] = atoms[i];
	*nitems = kept;
	return rc;
}
