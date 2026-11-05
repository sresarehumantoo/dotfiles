# Shared JWT crypto helpers for jwt-decode and jwt-encode.
# Loaded with `do "<dir>/_jwt.pl"` by the perl those scripts invoke.
# Only base64url and the ECDSA raw<->DER conversion live here, since that
# conversion is the one piece subtle enough that a second copy would drift.
use strict;
use warnings;
use MIME::Base64  qw(encode_base64 decode_base64);

sub b64u_encode {
    my ($bytes) = @_;
    my $s = encode_base64($bytes, "");
    $s =~ tr{+/}{-_};
    $s =~ s/=+$//;
    return $s;
}

sub b64u_decode {
    my ($s) = @_;
    $s =~ tr{-_}{+/};
    $s .= "=" x ((4 - length($s) % 4) % 4);
    return decode_base64($s);
}

sub _der_len {
    my ($n) = @_;
    return chr($n) if $n < 128;
    my $b = "";
    while ($n > 0) { $b = chr($n & 0xff) . $b; $n >>= 8; }
    return chr(0x80 | length($b)) . $b;
}

sub _der_int {
    my ($v) = @_;
    $v =~ s/^\x00+//;
    $v = "\x00" if $v eq "";
    $v = "\x00" . $v if ord(substr($v, 0, 1)) & 0x80;
    return "\x02" . _der_len(length $v) . $v;
}

sub _read_len {
    my ($s, $i) = @_;
    my $b = ord(substr($s, $i, 1));
    return ($b, 1) if $b < 128;
    my $n = $b & 0x7f;
    my $len = 0;
    $len = ($len << 8) | ord(substr($s, $i + 1 + $_, 1)) for (0 .. $n - 1);
    return ($len, 1 + $n);
}

# JOSE ECDSA signatures are the raw r||s, each integer left-padded to the curve
# size; openssl speaks the DER SEQUENCE { INTEGER r, INTEGER s }.
sub ecdsa_raw_to_der {
    my ($raw) = @_;
    die "jwt: odd ECDSA signature length\n" if length($raw) % 2;
    my $half = length($raw) / 2;
    my $seq = _der_int(substr($raw, 0, $half)) . _der_int(substr($raw, $half));
    return "\x30" . _der_len(length $seq) . $seq;
}

sub ecdsa_der_to_raw {
    my ($der, $intlen) = @_;
    my $i = 0;
    die "jwt: bad ECDSA DER\n" unless ord(substr($der, $i++, 1)) == 0x30;
    (undef, my $adv) = _read_len($der, $i);
    $i += $adv;
    my @ints;
    for (1 .. 2) {
        die "jwt: bad ECDSA DER\n" unless ord(substr($der, $i++, 1)) == 0x02;
        (my $len, $adv) = _read_len($der, $i);
        $i += $adv;
        my $v = substr($der, $i, $len);
        $i += $len;
        $v =~ s/^\x00+//;
        die "jwt: ECDSA integer too long\n" if length($v) > $intlen;
        $v = ("\x00" x ($intlen - length $v)) . $v;
        push @ints, $v;
    }
    return join "", @ints;
}

# ES256->32, ES384->48, ES512->66 (P-521 r/s are 66 bytes).
sub ecdsa_int_len {
    my ($alg) = @_;
    return 32 if $alg eq "ES256";
    return 48 if $alg eq "ES384";
    return 66 if $alg eq "ES512";
    die "jwt: unknown ECDSA alg $alg\n";
}

1;
