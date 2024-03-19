The x25519ell2 package provides X25519 obfuscated with Elligator 2, with
special care taken to handle cofactor related issues, and fixes for the
bugs in agl's original Elligator2 implementation.

All existing versions prior to the migration to the new code (anything
that uses agl's code) are fatally broken, and trivial to distinguish via
some simple math.  For more details see Loup Vaillant's writings on the
subject.  Any bugs in the implementation are mine, and not his.

Representatives created by this implementation will correctly be decoded
by existing implementations.  Public keys created by this implementation
be it via the modified scalar basepoint multiply or via decoding a
representative will be somewhat non-standard, but will interoperate with
a standard X25519 scalar-multiply.

As the representative to public key transform should be identical,
this change is fully-backward compatible (though the non-upgraded side
of the connection will still be trivially distinguishable from random).
